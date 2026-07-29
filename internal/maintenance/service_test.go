package maintenance

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/indexstate"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestDailyRemovesOnlyOldUnreferencedFilesAndReportsMissingDatabaseImages(t *testing.T) {
	directory, service, db := prepareMaintenanceTest(t)
	defer db.Close()
	now := time.Date(2026, time.July, 29, 3, 0, 0, 0, time.UTC)
	missing := maintenanceImage("019c1234-5678-7abc-8def-0123456789a1", "missing-image")
	if err := images.NewRepository(db).Create(context.Background(), missing); err != nil {
		t.Fatalf("create missing image record: %v", err)
	}
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-time.Hour)
	writeAgedFile(t, filepath.Join(directory, "images", "old-orphan"), old)
	writeAgedFile(t, filepath.Join(directory, "images", "recent-orphan"), recent)
	writeAgedFile(t, filepath.Join(directory, "cache", "thumbnails", "old-thumbnail"), old)
	writeAgedFile(t, filepath.Join(directory, "cache", "thumbnails", "recent-thumbnail"), recent)
	writeAgedFile(t, filepath.Join(directory, "tmp", "old-temporary"), old)
	writeAgedFile(t, filepath.Join(directory, "tmp", "recent-temporary"), recent)

	result, err := service.Daily(context.Background(), now, 24*time.Hour)
	if err != nil {
		t.Fatalf("Daily() error = %v", err)
	}
	if result.RemovedOrphanImages != 1 || result.RemovedOrphanThumbnails != 1 || result.RemovedTemporaryFiles != 1 || result.CleanupFailures != 0 {
		t.Fatalf("Daily() cleanup = %+v", result)
	}
	if result.Inspection.MissingImageCount != 1 || len(result.Inspection.MissingImageIDs) != 1 ||
		result.Inspection.OrphanImageCount != 1 || result.Inspection.OrphanThumbnailCount != 1 || result.Inspection.TemporaryFiles != 1 {
		t.Fatalf("Daily() inspection = %+v", result.Inspection)
	}
	for _, path := range []string{
		filepath.Join(directory, "images", "old-orphan"),
		filepath.Join(directory, "cache", "thumbnails", "old-thumbnail"),
		filepath.Join(directory, "tmp", "old-temporary"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("old unreferenced file remained: %s, error=%v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(directory, "images", "recent-orphan"),
		filepath.Join(directory, "cache", "thumbnails", "recent-thumbnail"),
		filepath.Join(directory, "tmp", "recent-temporary"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("recent file was removed: %s, error=%v", path, err)
		}
	}
	var imageCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM images WHERE id = ?", missing.ID).Scan(&imageCount); err != nil || imageCount != 1 {
		t.Fatalf("missing database image was deleted: count=%d error=%v", imageCount, err)
	}
	overview, err := service.Overview(context.Background())
	if err != nil || overview.MissingImageCount != 1 || overview.LastDaily == nil || overview.Indexes.Images != 0 {
		t.Fatalf("Overview() = %+v, %v", overview, err)
	}
}

func TestDailyRetriesCleanupFailureOnNextRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions differ on Windows")
	}
	directory, service, db := prepareMaintenanceTest(t)
	defer db.Close()
	now := time.Date(2026, time.July, 29, 3, 0, 0, 0, time.UTC)
	imagesDirectory := filepath.Join(directory, "images")
	path := filepath.Join(imagesDirectory, "retry-orphan")
	writeAgedFile(t, path, now.Add(-48*time.Hour))
	if err := os.Chmod(imagesDirectory, 0o550); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(imagesDirectory, 0o750) })
	first, err := service.Daily(context.Background(), now, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(imagesDirectory, 0o750); err != nil {
		t.Fatal(err)
	}
	if first.CleanupFailures != 1 {
		t.Fatalf("first cleanup failures = %d", first.CleanupFailures)
	}
	second, err := service.Daily(context.Background(), now.Add(time.Minute), 24*time.Hour)
	if err != nil || second.RemovedOrphanImages != 1 || second.CleanupFailures != 0 {
		t.Fatalf("second Daily() = %+v, %v", second, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("retried orphan remained: %v", err)
	}
}

func prepareMaintenanceTest(t *testing.T) (string, *Service, *sql.DB) {
	t.Helper()
	directory := t.TempDir()
	for _, path := range []string{"db", "images", filepath.Join("cache", "thumbnails"), "tmp"} {
		if err := os.MkdirAll(filepath.Join(directory, path), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	db, err := database.Open(filepath.Join(directory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	barrier := indexbarrier.New()
	authRepository := auth.NewRepository(db)
	sessionIndex := auth.NewSessionIndex()
	authService, err := auth.NewServiceWithBarrier(authRepository, sessionIndex, barrier)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	tokenRepository := apitoken.NewRepository(db)
	tokenIndex := apitoken.NewIndex()
	tokenService := apitoken.NewServiceWithBarrier(tokenRepository, tokenIndex, barrier)
	filesystem := storage.NewFilesystem(directory)
	deliveryIndex := delivery.NewIndex()
	rebuilder := indexstate.NewRebuilder(db, filesystem, authRepository, tokenRepository, deliveryIndex, sessionIndex, tokenIndex, barrier)
	var logs bytes.Buffer
	service := NewService(NewRepository(db), filesystem, rebuilder, deliveryIndex, authService, tokenService, slog.New(slog.NewJSONHandler(&logs, nil)))
	return directory, service, db
}

func maintenanceImage(id, storageKey string) images.Image {
	var digest [32]byte
	digest[0] = 1
	return images.Image{
		ID: id, OriginalName: "missing.jpg", StorageKey: storageKey, Extension: ".jpg", MIMEType: "image/jpeg",
		Width: 1, Height: 1, SourceSize: 1, StoredSize: 1, SourceSHA256: digest, StoredSHA256: digest,
		ProcessingSummary: "{}", Visibility: images.VisibilityPublic, UploadedVia: "admin", CreatedAt: time.Now().UTC(),
	}
}

func writeAgedFile(t *testing.T, path string, modified time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}
