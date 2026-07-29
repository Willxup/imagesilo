package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	imagealias "github.com/Willxup/imagesilo/internal/alias"
	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/config"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/httpapi"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/importer"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/indexstate"
	"github.com/Willxup/imagesilo/internal/maintenance"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/Willxup/imagesilo/internal/settings"
	"github.com/Willxup/imagesilo/internal/webui"
)

type Application struct {
	Handler http.Handler
	cancel  context.CancelFunc
}

func Build(ctx context.Context, cfg config.Config, db *sql.DB, logger *slog.Logger) (*Application, error) {
	applicationContext, cancel := context.WithCancel(ctx)
	filesystem := storage.NewFilesystem(cfg.DataDirectory)
	barrier := indexbarrier.New()
	deliveryIndex := delivery.NewIndex()
	authRepository := auth.NewRepository(db)
	sessionIndex := auth.NewSessionIndex()
	authService, err := auth.NewServiceWithBarrier(authRepository, sessionIndex, barrier)
	if err != nil {
		cancel()
		return nil, err
	}
	if _, err := authService.CleanupExpired(ctx, time.Now()); err != nil {
		cancel()
		return nil, err
	}
	tokenRepository := apitoken.NewRepository(db)
	tokenIndex := apitoken.NewIndex()
	tokenService := apitoken.NewServiceWithBarrier(tokenRepository, tokenIndex, barrier)
	rebuilder := indexstate.NewRebuilder(
		db, filesystem, authRepository, tokenRepository,
		deliveryIndex, sessionIndex, tokenIndex, barrier,
	)
	loadResult, err := rebuilder.Rebuild(ctx, time.Now())
	if err != nil {
		cancel()
		return nil, err
	}
	for _, id := range loadResult.Delivery.MissingIDs {
		logger.Error("image file is missing and was excluded from delivery index", "image_id", id)
	}

	engine := processor.NewEngine()
	gate := processor.NewGate(cfg.ProcessingConcurrency)
	imageRepository := images.NewRepository(db)
	imageService := images.NewServiceWithProcessorAndBarrier(
		imageRepository, filesystem, deliveryIndex,
		engine, gate,
		barrier,
	)
	importService := importer.NewService(importer.NewRepository(db), filesystem, deliveryIndex, engine, gate, barrier)
	aliasService := imagealias.NewService(imagealias.NewRepository(db), deliveryIndex, barrier)
	settingsService := settings.NewService(settings.NewRepository(db))
	currentSettings, err := settingsService.Get(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("load application settings: %w", err)
	}
	maintenanceService := maintenance.NewService(
		maintenance.NewRepository(db), filesystem, rebuilder, deliveryIndex, authService, tokenService, logger,
	)
	maintenanceService.RecordStartupMissing(loadResult.Delivery.MissingIDs)
	ui, err := webui.New()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("initialize web UI: %w", err)
	}

	application := &Application{cancel: cancel}
	application.Handler = httpapi.NewRouter(httpapi.Dependencies{
		DB:                    db,
		Logger:                logger,
		Auth:                  authService,
		APITokens:             tokenService,
		Aliases:               aliasService,
		Images:                imageService,
		Importer:              importService,
		Settings:              settingsService,
		Maintenance:           maintenanceService,
		DeliveryIndex:         deliveryIndex,
		Storage:               filesystem,
		CookieSecure:          cfg.CookieSecure,
		ProcessingConcurrency: cfg.ProcessingConcurrency,
		UI:                    ui,
	})
	go runMaintenance(applicationContext, logger, authService, tokenService, maintenanceService, currentSettings.MaintenanceHour)
	return application, nil
}

func (a *Application) Close() {
	a.cancel()
}

func runMaintenance(
	ctx context.Context,
	logger *slog.Logger,
	sessions *auth.Service,
	tokens *apitoken.Service,
	maintenanceService *maintenance.Service,
	maintenanceHour int,
) {
	authTicker := time.NewTicker(time.Minute)
	dailyTimer := time.NewTimer(untilNextMaintenance(time.Now(), maintenanceHour))
	defer authTicker.Stop()
	defer dailyTimer.Stop()
	if _, _, err := maintenanceService.CleanupStaleTemporary(time.Now(), 24*time.Hour); err != nil {
		logger.Error("startup temporary cleanup failed", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-authTicker.C:
			deletedSessions, err := sessions.CleanupExpired(ctx, now)
			if err != nil {
				logger.Error("expired session cleanup failed", "error", err)
				continue
			}
			removedTokens := tokens.CleanupExpired(now)
			if deletedSessions > 0 || removedTokens > 0 {
				logger.Info("expired authentication entries removed",
					"sessions", deletedSessions,
					"api_tokens", removedTokens,
				)
			}
		case now := <-dailyTimer.C:
			if _, err := maintenanceService.Daily(ctx, now, 24*time.Hour); err != nil {
				logger.Error("daily maintenance failed", "error", err)
			}
			dailyTimer.Reset(untilNextMaintenance(time.Now(), maintenanceHour))
		}
	}
}

func untilNextMaintenance(now time.Time, hour int) time.Duration {
	if hour < 0 || hour > 23 {
		hour = 3
	}
	utc := now.UTC()
	next := time.Date(utc.Year(), utc.Month(), utc.Day(), hour, 0, 0, 0, time.UTC)
	if !next.After(utc) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(utc)
}
