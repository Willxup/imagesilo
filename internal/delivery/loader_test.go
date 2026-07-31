package delivery

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestBuildLoadsAliasesAndSkipsAliasesForMissingFiles(t *testing.T) {
	dataDirectory, db := prepareLoaderTest(t)
	defer db.Close()
	const liveID = "019c1234-5678-7abc-8def-0123456789ab"
	const missingID = "019c1234-5678-7abc-8def-0123456789ac"
	insertDeliveryTestImage(t, db, liveID)
	insertDeliveryTestImage(t, db, missingID)
	if err := os.WriteFile(filepath.Join(dataDirectory, "images", liveID), []byte("image"), 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	insertDeliveryTestAlias(t, db, "019c1234-5678-7abc-8def-0123456789ad", "/legacy/live.jpg", liveID)
	insertDeliveryTestAlias(t, db, "019c1234-5678-7abc-8def-0123456789ae", "/legacy/missing.jpg", missingID)

	snapshot, result, err := Build(context.Background(), db, storage.NewFilesystem(dataDirectory))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.LoadedIDs) != 1 || len(result.MissingIDs) != 1 || result.LoadedAliasCount != 1 || result.SkippedAliasCount != 1 {
		t.Fatalf("Build() result = %+v", result)
	}
	index := NewIndex()
	index.ReplaceAll(snapshot.Targets, snapshot.Aliases)
	if target, ok := index.GetAlias("/legacy/live.jpg"); !ok || target.StorageKey != liveID {
		t.Fatalf("live alias target = %+v, %t", target, ok)
	}
	if _, ok := index.GetAlias("/legacy/missing.jpg"); ok {
		t.Fatal("alias for missing image entered the delivery index")
	}
}

func TestBuildRejectsReservedAliasStoredOutsideService(t *testing.T) {
	dataDirectory, db := prepareLoaderTest(t)
	defer db.Close()
	const imageID = "019c1234-5678-7abc-8def-0123456789ab"
	insertDeliveryTestImage(t, db, imageID)
	if err := os.WriteFile(filepath.Join(dataDirectory, "images", imageID), []byte("image"), 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	insertDeliveryTestAlias(t, db, "019c1234-5678-7abc-8def-0123456789ad", "/admin/settings", imageID)
	if _, _, err := Build(context.Background(), db, storage.NewFilesystem(dataDirectory)); !errors.Is(err, ErrReservedAliasPath) {
		t.Fatalf("Build() error = %v, want ErrReservedAliasPath", err)
	}
}

func TestBuildExcludesStoredFileWithUnexpectedSize(t *testing.T) {
	dataDirectory, db := prepareLoaderTest(t)
	defer db.Close()
	const imageID = "019c1234-5678-7abc-8def-0123456789ab"
	insertDeliveryTestImage(t, db, imageID)
	if err := os.WriteFile(filepath.Join(dataDirectory, "images", imageID), []byte("truncated"), 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	snapshot, result, err := Build(context.Background(), db, storage.NewFilesystem(dataDirectory))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.InvalidSizeIDs) != 1 || result.InvalidSizeIDs[0] != imageID || len(snapshot.Targets) != 0 {
		t.Fatalf("Build() result = %+v, targets = %+v", result, snapshot.Targets)
	}
}

func prepareLoaderTest(t *testing.T) (string, *sql.DB) {
	t.Helper()
	directory := t.TempDir()
	for _, path := range []string{"db", "images", filepath.Join("cache", "thumbnails"), "tmp"} {
		if err := os.MkdirAll(filepath.Join(directory, path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", path, err)
		}
	}
	db, err := database.Open(filepath.Join(directory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	return directory, db
}

func insertDeliveryTestImage(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	hash := make([]byte, 32)
	if _, err := db.Exec(`
		INSERT INTO images(
			id, original_name, storage_key, extension, mime_type, width, height,
			source_size, stored_size, source_sha256, stored_sha256, processing_summary,
			visibility, uploaded_via, uploaded_by_api_token_id, created_at
		) VALUES (?, 'test.jpg', ?, '.jpg', 'image/jpeg', 1, 1, 5, 5, ?, ?, '{}', 'public', 'admin', NULL, ?)`,
		id, id, hash, hash, time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert image: %v", err)
	}
}

func insertDeliveryTestAlias(t *testing.T, db *sql.DB, id, path, imageID string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO image_aliases(id, alias_path, image_id, source, created_at)
		VALUES (?, ?, ?, 'test', ?)`, id, path, imageID, time.Now().Unix()); err != nil {
		t.Fatalf("insert alias: %v", err)
	}
}
