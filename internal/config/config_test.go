package config

import (
	"os"
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
		t.Fatalf("ProcessingConcurrency = %d, want lightweight default 1", cfg.ProcessingConcurrency)
	}
}

func TestPrepareDataDirectoriesIncludesMigrationMount(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "data")
	cfg := Config{DataDirectory: dataDirectory}
	if err := cfg.PrepareDataDirectories(); err != nil {
		t.Fatalf("PrepareDataDirectories() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(dataDirectory, "migrations"))
	if err != nil {
		t.Fatalf("Stat(migrations) error = %v", err)
	}
	if !info.IsDir() {
		t.Fatal("migrations path is not a directory")
	}
}

func TestLoadRejectsRelativeDataDirectory(t *testing.T) {
	t.Setenv("IMAGESILO_DATA_DIR", "relative")
	if _, err := Load(); err == nil {
		t.Fatal("Load() unexpectedly accepted a relative data directory")
	}
}
