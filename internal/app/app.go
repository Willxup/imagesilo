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
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/indexstate"
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

	imageRepository := images.NewRepository(db)
	imageService := images.NewServiceWithProcessorAndBarrier(
		imageRepository, filesystem, deliveryIndex,
		processor.NewEngine(), processor.NewGate(cfg.ProcessingConcurrency),
		barrier,
	)
	aliasService := imagealias.NewService(imagealias.NewRepository(db), deliveryIndex, barrier)
	settingsService := settings.NewService(settings.NewRepository(db))
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
		Settings:              settingsService,
		DeliveryIndex:         deliveryIndex,
		Storage:               filesystem,
		CookieSecure:          cfg.CookieSecure,
		ProcessingConcurrency: cfg.ProcessingConcurrency,
		UI:                    ui,
	})
	go runExpirationCleanup(applicationContext, logger, authService, tokenService)
	return application, nil
}

func (a *Application) Close() {
	a.cancel()
}

func runExpirationCleanup(ctx context.Context, logger *slog.Logger, sessions *auth.Service, tokens *apitoken.Service) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
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
		}
	}
}
