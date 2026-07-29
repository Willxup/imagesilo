package migrations

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Willxup/imagesilo/internal/platform/database"
)

func TestApplyIsIdempotent(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()

	for i := 0; i < 2; i++ {
		if err := Apply(context.Background(), db); err != nil {
			t.Fatalf("Apply() pass %d error = %v", i+1, err)
		}
	}

	var count int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 4 {
		t.Fatalf("migration count = %d, want 4", count)
	}
	var maxTotalPixels int64
	if err := db.QueryRow("SELECT max_total_pixels FROM app_settings WHERE singleton = 1").Scan(&maxTotalPixels); err != nil {
		t.Fatalf("read max_total_pixels: %v", err)
	}
	if maxTotalPixels != 16_000_000 {
		t.Fatalf("max_total_pixels = %d, want 16000000", maxTotalPixels)
	}
}

func TestDefaultProcessingLimitMigrationOnlyChangesTheOldDefault(t *testing.T) {
	for _, test := range []struct {
		name     string
		before   int64
		expected int64
	}{
		{name: "old default", before: 40_000_000, expected: 16_000_000},
		{name: "custom value", before: 12_000_000, expected: 12_000_000},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
			if err != nil {
				t.Fatalf("database.Open() error = %v", err)
			}
			defer db.Close()
			if err := Apply(context.Background(), db); err != nil {
				t.Fatalf("initial Apply() error = %v", err)
			}
			if _, err := db.Exec("UPDATE app_settings SET max_total_pixels = ? WHERE singleton = 1", test.before); err != nil {
				t.Fatalf("set pre-migration value: %v", err)
			}
			if _, err := db.Exec("DELETE FROM schema_migrations WHERE version = ?", "0004_default_processing_limits.up.sql"); err != nil {
				t.Fatalf("rewind migration marker: %v", err)
			}
			if err := Apply(context.Background(), db); err != nil {
				t.Fatalf("reapply migration: %v", err)
			}
			var actual int64
			if err := db.QueryRow("SELECT max_total_pixels FROM app_settings WHERE singleton = 1").Scan(&actual); err != nil {
				t.Fatalf("read migrated value: %v", err)
			}
			if actual != test.expected {
				t.Fatalf("max_total_pixels = %d, want %d", actual, test.expected)
			}
		})
	}
}
