package command

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/app"
	"github.com/Willxup/imagesilo/internal/config"
	"github.com/Willxup/imagesilo/internal/maintenance"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/processor"
)

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.PrepareDataDirectories(); err != nil {
		return err
	}

	db, err := database.Open(cfg.DatabasePath())
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrations.Apply(context.Background(), db); err != nil {
		return err
	}
	if err := processor.Startup(); err != nil {
		return err
	}
	defer processor.Shutdown()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	application, err := app.Build(context.Background(), cfg, db, logger)
	if err != nil {
		return err
	}
	defer application.Close()
	snapshot := maintenance.CaptureRuntime()
	logger.Info("runtime startup baseline",
		"go_heap_alloc_bytes", snapshot.HeapAllocBytes,
		"go_heap_sys_bytes", snapshot.HeapSysBytes,
		"goroutines", snapshot.Goroutines,
	)
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           application.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server starting", "address", cfg.ListenAddress)
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case sig := <-signals:
		logger.Info("shutdown requested", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return server.Shutdown(ctx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
