package image

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestImageSearchUsesKeysetPaginationAndAllFilters(t *testing.T) {
	directory, db, service, index := prepareManagementTest(t)
	defer db.Close()
	base := time.Unix(1_700_000_000, 0).UTC()
	alpha := managementImage("019c1234-5678-7abc-8def-0123456789a1", "Alpha Photo.jpg", "image/jpeg", VisibilityPublic, "admin", 900, 800, 600, base.Add(3*time.Hour), 0xaa)
	beta := managementImage("019c1234-5678-7abc-8def-0123456789a2", "Beta.png", "image/png", VisibilityPrivate, "api_token", 2_000, 1200, 900, base.Add(2*time.Hour), 0xbb)
	gamma := managementImage("019c1234-5678-7abc-8def-0123456789a3", "Gamma.webp", "image/webp", VisibilityPublic, "import", 300, 320, 240, base.Add(time.Hour), 0xcc)
	for _, value := range []Image{alpha, beta, gamma} {
		insertManagementImage(t, directory, db, index, value)
	}
	if _, err := db.Exec(`INSERT INTO image_aliases(id, alias_path, image_id, source, created_at) VALUES (?, ?, ?, 'test', ?)`,
		"alias-beta", "/legacy/beta.png", beta.ID, base.Unix()); err != nil {
		t.Fatalf("insert alias: %v", err)
	}
	index.AddAlias("/legacy/beta.png", beta.ID)

	first, err := service.Search(context.Background(), ListFilter{Limit: 2})
	if err != nil || len(first.Items) != 2 || first.Items[0].ID != alpha.ID || first.Items[1].ID != beta.ID || first.NextCursor == "" {
		t.Fatalf("first page = %+v, error = %v", first, err)
	}
	second, err := service.Search(context.Background(), ListFilter{Limit: 2, Cursor: first.NextCursor})
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != gamma.ID || second.NextCursor != "" {
		t.Fatalf("second page = %+v, error = %v", second, err)
	}

	tests := []struct {
		name   string
		filter ListFilter
		wantID string
	}{
		{"filename", ListFilter{Query: "alpha photo"}, alpha.ID},
		{"alias path", ListFilter{Query: "legacy/beta"}, beta.ID},
		{"hash", ListFilter{Query: hex.EncodeToString(beta.StoredSHA256[:])}, beta.ID},
		{"visibility", ListFilter{Visibility: VisibilityPrivate}, beta.ID},
		{"format", ListFilter{MIMEType: "image/webp"}, gamma.ID},
		{"size", ListFilter{MinStoredBytes: 1_500, MaxStoredBytes: 2_500}, beta.ID},
		{"dimensions", ListFilter{MinWidth: 1000, MinHeight: 800}, beta.ID},
		{"date", ListFilter{CreatedFrom: timePointer(base.Add(90 * time.Minute)), CreatedTo: timePointer(base.Add(150 * time.Minute))}, beta.ID},
		{"upload source", ListFilter{UploadedVia: "import"}, gamma.ID},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, err := service.Search(context.Background(), test.filter)
			if err != nil || len(page.Items) != 1 || page.Items[0].ID != test.wantID {
				t.Fatalf("Search(%+v) = %+v, %v", test.filter, page, err)
			}
		})
	}
	if _, err := service.Search(context.Background(), ListFilter{Cursor: "invalid"}); !errors.Is(err, ErrInvalidListFilter) {
		t.Fatalf("invalid cursor error = %v", err)
	}
	if value, err := service.Get(context.Background(), beta.ID); err != nil || value.ID != beta.ID {
		t.Fatalf("Get() = %+v, %v", value, err)
	}
}

func TestPermanentDeleteCommitsDatabaseThenRemovesIndexAndFiles(t *testing.T) {
	directory, db, service, index := prepareManagementTest(t)
	defer db.Close()
	value := managementImage("019c1234-5678-7abc-8def-0123456789a1", "delete.jpg", "image/jpeg", VisibilityPublic, "admin", 10, 10, 10, time.Now(), 0xdd)
	insertManagementImage(t, directory, db, index, value)
	if _, err := db.Exec(`INSERT INTO image_aliases(id, alias_path, image_id, source, created_at) VALUES ('delete-alias', '/legacy/delete.jpg', ?, 'test', ?)`, value.ID, time.Now().Unix()); err != nil {
		t.Fatalf("insert alias: %v", err)
	}
	index.AddAlias("/legacy/delete.jpg", value.ID)
	thumbnailPath := filepath.Join(directory, "cache", "thumbnails", value.ID)
	if err := os.WriteFile(thumbnailPath, []byte("thumbnail"), 0o640); err != nil {
		t.Fatalf("write thumbnail: %v", err)
	}

	result, err := service.Delete(context.Background(), value.ID)
	if err != nil || result.CleanupPending || !result.ImageFileDeleted || !result.ThumbnailDeleted {
		t.Fatalf("Delete() = %+v, %v", result, err)
	}
	if _, err := service.Get(context.Background(), value.ID); !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if _, ok := index.Get(value.ID); ok {
		t.Fatal("deleted image remained in delivery index")
	}
	if _, ok := index.ResolveAlias("/legacy/delete.jpg"); ok {
		t.Fatal("cascaded alias remained in delivery index")
	}
	var aliases int
	if err := db.QueryRow("SELECT COUNT(*) FROM image_aliases WHERE image_id = ?", value.ID).Scan(&aliases); err != nil || aliases != 0 {
		t.Fatalf("remaining aliases = %d, error = %v", aliases, err)
	}
	for _, path := range []string{filepath.Join(directory, "images", value.StorageKey), thumbnailPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("deleted file %s stat error = %v", filepath.Base(path), err)
		}
	}
}

func TestPermanentDeleteReportsCleanupPendingWithoutRestoringDatabase(t *testing.T) {
	directory, db, service, index := prepareManagementTest(t)
	defer db.Close()
	value := managementImage("019c1234-5678-7abc-8def-0123456789a1", "blocked.jpg", "image/jpeg", VisibilityPublic, "admin", 10, 10, 10, time.Now(), 0xee)
	if err := NewRepository(db).Create(context.Background(), value); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	blockedPath := filepath.Join(directory, "images", value.StorageKey)
	if err := os.Mkdir(blockedPath, 0o750); err != nil {
		t.Fatalf("Mkdir(blocked image): %v", err)
	}
	if err := os.WriteFile(filepath.Join(blockedPath, "child"), []byte("block"), 0o640); err != nil {
		t.Fatalf("WriteFile(blocker): %v", err)
	}
	index.Add(value.ID, delivery.Target{StorageKey: value.StorageKey})

	result, err := service.Delete(context.Background(), value.ID)
	if err != nil || !result.CleanupPending || result.ImageFileDeleted || result.ImageCleanupError == nil {
		t.Fatalf("Delete() = %+v, %v", result, err)
	}
	if _, err := service.Get(context.Background(), value.ID); !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("database record was restored after cleanup failure: %v", err)
	}
	if _, ok := index.Get(value.ID); ok {
		t.Fatal("index entry was restored after cleanup failure")
	}
}

func prepareManagementTest(t *testing.T) (string, *sql.DB, *Service, *delivery.Index) {
	t.Helper()
	directory := prepareUploadTestData(t)
	db, err := database.Open(filepath.Join(directory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	index := delivery.NewIndex()
	service := NewServiceWithProcessorAndBarrier(
		NewRepository(db), storage.NewFilesystem(directory), index,
		processor.NewEngine(), processor.NewGate(1), indexbarrier.New(),
	)
	return directory, db, service, index
}

func managementImage(id, name, mime string, visibility Visibility, uploadedVia string, size int64, width, height int, createdAt time.Time, hashByte byte) Image {
	var hash [32]byte
	for index := range hash {
		hash[index] = hashByte
	}
	extension := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mime]
	return Image{
		ID: id, OriginalName: name, StorageKey: id, Extension: extension, MIMEType: mime,
		Width: width, Height: height, SourceSize: size, StoredSize: size,
		SourceSHA256: hash, StoredSHA256: hash, ProcessingSummary: `{}`,
		Visibility: visibility, UploadedVia: uploadedVia, CreatedAt: createdAt.UTC(),
	}
}

func insertManagementImage(t *testing.T, directory string, db *sql.DB, index *delivery.Index, value Image) {
	t.Helper()
	if err := NewRepository(db).Create(context.Background(), value); err != nil {
		t.Fatalf("Create(%s) error = %v", value.ID, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "images", value.StorageKey), []byte("image-data"), 0o640); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", value.ID, err)
	}
	index.Add(value.ID, delivery.Target{StorageKey: value.StorageKey, Visibility: string(value.Visibility)})
}

func timePointer(value time.Time) *time.Time {
	return &value
}
