package imagealias

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/platform/database"
)

const aliasTestImageID = "019c1234-5678-7abc-8def-0123456789ab"

func TestAliasLifecycleCommitsDatabaseBeforeIndex(t *testing.T) {
	db := openAliasTestDatabase(t)
	defer db.Close()
	insertAliasTestImage(t, db, aliasTestImageID)

	index := delivery.NewIndex()
	index.Add(aliasTestImageID, delivery.Target{StorageKey: aliasTestImageID})
	service := NewService(NewRepository(db), index, indexbarrier.New())
	now := time.Unix(1_700_000_000, 0).UTC()
	created, err := service.Create(context.Background(), "/legacy/旧图.jpg", aliasTestImageID, " legacy-import ", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Path != "/legacy/%E6%97%A7%E5%9B%BE.jpg" || created.Source != "legacy-import" {
		t.Fatalf("created alias = %+v", created)
	}
	if imageID, ok := index.ResolveAlias(created.Path); !ok || imageID != aliasTestImageID {
		t.Fatalf("index alias = %q, %t", imageID, ok)
	}

	duplicate, err := service.Create(context.Background(), created.Path, aliasTestImageID, "duplicate", now.Add(time.Second))
	if !errors.Is(err, ErrAliasConflict) || duplicate.ID != "" {
		t.Fatalf("duplicate Create() = %+v, %v", duplicate, err)
	}
	resolved, err := service.Resolve(context.Background(), "/legacy/旧图.jpg")
	if err != nil || resolved.ID != created.ID {
		t.Fatalf("Resolve() = %+v, %v", resolved, err)
	}
	values, err := service.List(context.Background(), 10)
	if err != nil || len(values) != 1 || values[0].ID != created.ID {
		t.Fatalf("List() = %+v, %v", values, err)
	}

	if err := service.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := index.ResolveAlias(created.Path); ok {
		t.Fatal("deleted alias remained in delivery index")
	}
	if _, err := service.Resolve(context.Background(), created.Path); !errors.Is(err, ErrAliasNotFound) {
		t.Fatalf("Resolve() after delete error = %v", err)
	}
}

func TestAliasValidationAndMissingTarget(t *testing.T) {
	db := openAliasTestDatabase(t)
	defer db.Close()
	service := NewService(NewRepository(db), delivery.NewIndex(), indexbarrier.New())
	now := time.Now()

	tests := []struct {
		path, imageID, source string
		wantErr               error
	}{
		{"/api/v1/images", aliasTestImageID, "test", delivery.ErrReservedAliasPath},
		{"/legacy/../image.jpg", aliasTestImageID, "test", delivery.ErrInvalidAliasPath},
		{"/legacy/image.jpg", "not-a-uuid", "test", ErrInvalidImage},
		{"/legacy/image.jpg", aliasTestImageID, "\n", ErrInvalidSource},
		{"/legacy/image.jpg", aliasTestImageID, "test", ErrImageNotFound},
	}
	for _, test := range tests {
		if _, err := service.Create(context.Background(), test.path, test.imageID, test.source, now); !errors.Is(err, test.wantErr) {
			t.Errorf("Create(%q, %q, %q) error = %v, want %v", test.path, test.imageID, test.source, err, test.wantErr)
		}
	}
}

func openAliasTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	return db
}

func insertAliasTestImage(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	hash := make([]byte, 32)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO images(
			id, original_name, storage_key, extension, mime_type, width, height,
			source_size, stored_size, source_sha256, stored_sha256, processing_summary,
			visibility, uploaded_via, uploaded_by_api_token_id, created_at
		) VALUES (?, 'test.jpg', ?, '.jpg', 'image/jpeg', 1, 1, 1, 1, ?, ?, '{}', 'public', 'admin', NULL, ?)`,
		id, id, hash, hash, time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert test image: %v", err)
	}
}
