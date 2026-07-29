package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/config"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/httpapi"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/Willxup/imagesilo/internal/webui"
)

type Application struct {
	Handler http.Handler
}

func Build(ctx context.Context, cfg config.Config, db *sql.DB, logger *slog.Logger) (*Application, error) {
	filesystem := storage.NewFilesystem(cfg.DataDirectory)
	deliveryIndex := delivery.NewIndex()
	loadResult, err := delivery.Load(ctx, db, filesystem, deliveryIndex)
	if err != nil {
		return nil, err
	}
	for _, id := range loadResult.MissingIDs {
		logger.Error("image file is missing and was excluded from delivery index", "image_id", id)
	}

	authRepository := auth.NewRepository(db)
	sessionIndex := auth.NewSessionIndex()
	authService, err := auth.NewService(authRepository, sessionIndex)
	if err != nil {
		return nil, err
	}
	if err := authService.LoadSessions(ctx, time.Now()); err != nil {
		return nil, err
	}

	imageRepository := images.NewRepository(db)
	imageService := images.NewService(imageRepository, filesystem, deliveryIndex)
	ui, err := webui.New()
	if err != nil {
		return nil, fmt.Errorf("initialize web UI: %w", err)
	}

	return &Application{Handler: httpapi.NewRouter(httpapi.Dependencies{
		DB:            db,
		Logger:        logger,
		Auth:          authService,
		Images:        imageService,
		DeliveryIndex: deliveryIndex,
		Storage:       filesystem,
		CookieSecure:  cfg.CookieSecure,
		UI:            ui,
	})}, nil
}
