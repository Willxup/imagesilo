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
	if count != 3 {
		t.Fatalf("migration count = %d, want 3", count)
	}
}
