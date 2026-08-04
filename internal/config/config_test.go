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
	t.Setenv("IMAGESILO_DELIVERY_CONCURRENCY", "")
	t.Setenv("IMAGESILO_SHUTDOWN_TIMEOUT", "")
	t.Setenv("IMAGESILO_TRUST_PROXY_HEADERS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ProcessingConcurrency != 1 {
		t.Fatalf("ProcessingConcurrency = %d, want lightweight default 1", cfg.ProcessingConcurrency)
	}
	if cfg.DeliveryConcurrency != 0 {
		t.Fatalf("DeliveryConcurrency = %d, want unlimited default 0", cfg.DeliveryConcurrency)
	}
	if !cfg.TrustProxyHeaders {
		t.Fatal("TrustProxyHeaders = false, want Nginx Proxy Manager default true")
	}
	if cfg.MigrationMutations {
		t.Fatal("MigrationMutations = true, want safe read-only default false")
	}
}

func TestLoadAcceptsExplicitMigrationMutations(t *testing.T) {
	t.Setenv("IMAGESILO_LISTEN_ADDRESS", "127.0.0.1:0")
	t.Setenv("IMAGESILO_DATA_DIR", filepath.Join(t.TempDir(), "data"))
	t.Setenv("IMAGESILO_MIGRATION_MUTATIONS", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.MigrationMutations {
		t.Fatal("MigrationMutations = false, want configured true")
	}
}

func TestNativeDefaultListensOnlyOnLoopback(t *testing.T) {
	t.Setenv("IMAGESILO_LISTEN_ADDRESS", "")
	t.Setenv("IMAGESILO_DATA_DIR", filepath.Join(t.TempDir(), "data"))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddress != "127.0.0.1:8080" {
		t.Fatalf("ListenAddress = %q, want loopback default", cfg.ListenAddress)
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

func TestLoadRejectsNegativeDeliveryConcurrency(t *testing.T) {
	t.Setenv("IMAGESILO_LISTEN_ADDRESS", "127.0.0.1:0")
	t.Setenv("IMAGESILO_DATA_DIR", filepath.Join(t.TempDir(), "data"))
	t.Setenv("IMAGESILO_DELIVERY_CONCURRENCY", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("Load() unexpectedly accepted negative delivery concurrency")
	}
}

func TestLoadAcceptsBoundedDeliveryConcurrency(t *testing.T) {
	t.Setenv("IMAGESILO_LISTEN_ADDRESS", "127.0.0.1:0")
	t.Setenv("IMAGESILO_DATA_DIR", filepath.Join(t.TempDir(), "data"))
	t.Setenv("IMAGESILO_DELIVERY_CONCURRENCY", "32")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DeliveryConcurrency != 32 {
		t.Fatalf("DeliveryConcurrency = %d, want configured limit 32", cfg.DeliveryConcurrency)
	}
}
