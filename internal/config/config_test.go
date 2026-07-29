package config

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesSafeDefaults(t *testing.T) {
	t.Setenv("IMAGESILO_LISTEN_ADDRESS", "127.0.0.1:0")
	t.Setenv("IMAGESILO_DATA_DIR", filepath.Join(t.TempDir(), "data"))
	t.Setenv("IMAGESILO_PROCESSING_CONCURRENCY", "")
	t.Setenv("IMAGESILO_SHUTDOWN_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ProcessingConcurrency != 1 {
		t.Fatalf("ProcessingConcurrency = %d, want 1 until benchmark selects a default", cfg.ProcessingConcurrency)
	}
}

func TestLoadRejectsRelativeDataDirectory(t *testing.T) {
	t.Setenv("IMAGESILO_DATA_DIR", "relative")
	if _, err := Load(); err == nil {
		t.Fatal("Load() unexpectedly accepted a relative data directory")
	}
}
