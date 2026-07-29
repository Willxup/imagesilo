package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	defaultListenAddress     = ":8080"
	defaultDataDirectory     = "/data"
	defaultProcessingWorkers = 4
	defaultShutdownTimeout   = 10 * time.Second
	minimumProcessingWorkers = 1
	maximumProcessingWorkers = 64
)

type Config struct {
	ListenAddress         string
	DataDirectory         string
	ProcessingConcurrency int
	ShutdownTimeout       time.Duration
	CookieSecure          bool
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddress:         envOrDefault("IMAGESILO_LISTEN_ADDRESS", defaultListenAddress),
		DataDirectory:         envOrDefault("IMAGESILO_DATA_DIR", defaultDataDirectory),
		ProcessingConcurrency: defaultProcessingWorkers,
		ShutdownTimeout:       defaultShutdownTimeout,
		CookieSecure:          true,
	}

	if raw := os.Getenv("IMAGESILO_PROCESSING_CONCURRENCY"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("IMAGESILO_PROCESSING_CONCURRENCY must be an integer: %w", err)
		}
		cfg.ProcessingConcurrency = value
	}
	if raw := os.Getenv("IMAGESILO_SHUTDOWN_TIMEOUT"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("IMAGESILO_SHUTDOWN_TIMEOUT must be a duration: %w", err)
		}
		cfg.ShutdownTimeout = value
	}
	if raw := os.Getenv("IMAGESILO_COOKIE_SECURE"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("IMAGESILO_COOKIE_SECURE must be true or false: %w", err)
		}
		cfg.CookieSecure = value
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if _, err := net.ResolveTCPAddr("tcp", c.ListenAddress); err != nil {
		return fmt.Errorf("invalid listen address %q: %w", c.ListenAddress, err)
	}
	if !filepath.IsAbs(c.DataDirectory) {
		return fmt.Errorf("IMAGESILO_DATA_DIR must be an absolute path")
	}
	if c.ProcessingConcurrency < minimumProcessingWorkers || c.ProcessingConcurrency > maximumProcessingWorkers {
		return fmt.Errorf("IMAGESILO_PROCESSING_CONCURRENCY must be between %d and %d", minimumProcessingWorkers, maximumProcessingWorkers)
	}
	if c.ShutdownTimeout <= 0 || c.ShutdownTimeout > time.Minute {
		return fmt.Errorf("IMAGESILO_SHUTDOWN_TIMEOUT must be greater than zero and at most 1m")
	}
	return nil
}

func (c Config) DatabasePath() string {
	return filepath.Join(c.DataDirectory, "db", "imagesilo.db")
}

func (c Config) PrepareDataDirectories() error {
	for _, path := range []string{
		filepath.Join(c.DataDirectory, "db"),
		filepath.Join(c.DataDirectory, "images"),
		filepath.Join(c.DataDirectory, "cache", "thumbnails"),
		filepath.Join(c.DataDirectory, "tmp"),
	} {
		if err := os.MkdirAll(path, 0o750); err != nil {
			return fmt.Errorf("prepare data directory %q: %w", path, err)
		}
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
