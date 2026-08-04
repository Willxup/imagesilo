package migrationimage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestSearchFlattensFiltersAndPaginatesMigrationImages(t *testing.T) {
	dataDirectory := prepareMigrationTestData(t)
	base := time.Unix(1_800_000_000, 0).UTC()
	jpegBytes := migrationTestJPEG(t)
	pngBytes := append(migrationTestPNG(t), make([]byte, 2048)...)
	writeMigrationTestImage(t, dataDirectory, "i/2026/new.jpg", jpegBytes, base.Add(3*time.Hour))
	writeMigrationTestImage(t, dataDirectory, "images/2025/旧图.png", pngBytes, base.Add(2*time.Hour))
	writeMigrationTestImage(t, dataDirectory, "images/2024/old.jpg", jpegBytes, base.Add(time.Hour))

	service := NewService(storage.NewFilesystem(dataDirectory), true)
	first, err := service.Search(context.Background(), ListFilter{Limit: 2})
	if err != nil || len(first.Items) != 2 || first.Items[0].Path != "/i/2026/new.jpg" || first.NextCursor == "" || !first.MutationsEnabled {
		t.Fatalf("first Search() = %+v, %v", first, err)
	}
	if first.Items[1].Path != "/images/2025/%E6%97%A7%E5%9B%BE.png" {
		t.Fatalf("canonical Unicode path = %q", first.Items[1].Path)
	}
	second, err := service.Search(context.Background(), ListFilter{Limit: 2, Cursor: first.NextCursor})
	if err != nil || len(second.Items) != 1 || second.Items[0].Path != "/images/2024/old.jpg" || second.NextCursor != "" {
		t.Fatalf("second Search() = %+v, %v", second, err)
	}

	filtered, err := service.Search(context.Background(), ListFilter{Query: "旧图", MIMEType: "image/png"})
	if err != nil || len(filtered.Items) != 1 || filtered.Items[0].OriginalName != "旧图.png" {
		t.Fatalf("filtered Search() = %+v, %v", filtered, err)
	}
	sized, err := service.Search(context.Background(), ListFilter{MinStoredBytes: int64(len(jpegBytes) + 1)})
	if err != nil || len(sized.Items) != 1 || sized.Items[0].MIMEType != "image/png" {
		t.Fatalf("size Search() = %+v, %v", sized, err)
	}
	if _, err := service.Search(context.Background(), ListFilter{Cursor: "invalid"}); !errors.Is(err, ErrInvalidListFilter) {
		t.Fatalf("invalid cursor error = %v", err)
	}
}

func TestDeleteRequiresCapabilityAndCleansEmptyDirectories(t *testing.T) {
	dataDirectory := prepareMigrationTestData(t)
	writeMigrationTestImage(t, dataDirectory, "i/2026/08/delete.jpg", migrationTestJPEG(t), time.Now())
	filesystem := storage.NewFilesystem(dataDirectory)

	readOnly := NewService(filesystem, false)
	if _, err := readOnly.Delete(context.Background(), "/i/2026/08/delete.jpg"); !errors.Is(err, ErrMutationsDisabled) {
		t.Fatalf("read-only Delete() error = %v", err)
	}
	enabled := NewService(filesystem, true)
	result, err := enabled.Delete(context.Background(), "/i/2026/08/delete.jpg")
	if err != nil || result.Path != "/i/2026/08/delete.jpg" || result.RemovedDirectories != 3 || result.DirectoryCleanupPending {
		t.Fatalf("Delete() = %+v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(dataDirectory, "migrations")); err != nil {
		t.Fatalf("migration root was removed: %v", err)
	}
	if _, err := enabled.Delete(context.Background(), "/i/2026/08/delete.jpg"); !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("second Delete() error = %v", err)
	}
	if _, err := enabled.Delete(context.Background(), "/api/v1/escape.jpg"); !errors.Is(err, ErrInvalidImagePath) {
		t.Fatalf("reserved Delete() error = %v", err)
	}
}

func TestSearchCachesUntilRefreshAndDeleteUpdatesSnapshot(t *testing.T) {
	dataDirectory := prepareMigrationTestData(t)
	base := time.Unix(1_800_000_000, 0).UTC()
	writeMigrationTestImage(t, dataDirectory, "i/first.jpg", migrationTestJPEG(t), base)
	service := NewService(storage.NewFilesystem(dataDirectory), true)

	initial, err := service.Search(context.Background(), ListFilter{})
	if err != nil || len(initial.Items) != 1 || initial.Items[0].Path != "/i/first.jpg" {
		t.Fatalf("initial Search() = %+v, %v", initial, err)
	}
	initialBytes, err := service.StoredBytes(context.Background())
	if err != nil || initialBytes != int64(len(migrationTestJPEG(t))) {
		t.Fatalf("initial StoredBytes() = %d, %v", initialBytes, err)
	}
	writeMigrationTestImage(t, dataDirectory, "images/second.jpg", migrationTestJPEG(t), base.Add(time.Hour))
	cached, err := service.Search(context.Background(), ListFilter{})
	if err != nil || len(cached.Items) != 1 {
		t.Fatalf("cached Search() = %+v, %v, want one cached image", cached, err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	refreshed, err := service.Search(context.Background(), ListFilter{})
	if err != nil || len(refreshed.Items) != 2 || refreshed.Items[0].Path != "/images/second.jpg" {
		t.Fatalf("refreshed Search() = %+v, %v", refreshed, err)
	}
	refreshedBytes, err := service.StoredBytes(context.Background())
	if err != nil || refreshedBytes != 2*initialBytes {
		t.Fatalf("refreshed StoredBytes() = %d, %v", refreshedBytes, err)
	}

	if _, err := service.Delete(context.Background(), "/images/second.jpg"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	afterDelete, err := service.Search(context.Background(), ListFilter{})
	if err != nil || len(afterDelete.Items) != 1 || afterDelete.Items[0].Path != "/i/first.jpg" {
		t.Fatalf("Search() after deletion = %+v, %v", afterDelete, err)
	}
	afterDeleteBytes, err := service.StoredBytes(context.Background())
	if err != nil || afterDeleteBytes != initialBytes {
		t.Fatalf("StoredBytes() after deletion = %d, %v", afterDeleteBytes, err)
	}
}

func TestDeleteEvictsExternallyRemovedImageFromSnapshot(t *testing.T) {
	dataDirectory := prepareMigrationTestData(t)
	writeMigrationTestImage(t, dataDirectory, "i/external.jpg", migrationTestJPEG(t), time.Now())
	service := NewService(storage.NewFilesystem(dataDirectory), true)

	initial, err := service.Search(context.Background(), ListFilter{})
	if err != nil || len(initial.Items) != 1 || initial.Items[0].Path != "/i/external.jpg" {
		t.Fatalf("initial Search() = %+v, %v", initial, err)
	}
	if err := os.Remove(filepath.Join(dataDirectory, "migrations", "i", "external.jpg")); err != nil {
		t.Fatalf("external Remove() error = %v", err)
	}
	if _, err := service.Delete(context.Background(), "/i/external.jpg"); !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("Delete() error = %v, want ErrImageNotFound", err)
	}
	afterDelete, err := service.Search(context.Background(), ListFilter{})
	if err != nil || len(afterDelete.Items) != 0 {
		t.Fatalf("Search() after external deletion = %+v, %v", afterDelete, err)
	}
}

func TestSearchRefreshesExpiredSnapshot(t *testing.T) {
	dataDirectory := prepareMigrationTestData(t)
	base := time.Unix(1_800_000_000, 0).UTC()
	writeMigrationTestImage(t, dataDirectory, "i/first.jpg", migrationTestJPEG(t), base)
	service := NewService(storage.NewFilesystem(dataDirectory), true)
	now := base
	service.now = func() time.Time { return now }

	if page, err := service.Search(context.Background(), ListFilter{}); err != nil || len(page.Items) != 1 {
		t.Fatalf("initial Search() = %+v, %v", page, err)
	}
	writeMigrationTestImage(t, dataDirectory, "i/second.jpg", migrationTestJPEG(t), base.Add(time.Hour))
	now = now.Add(snapshotTTL - time.Second)
	if page, err := service.Search(context.Background(), ListFilter{}); err != nil || len(page.Items) != 1 {
		t.Fatalf("fresh cached Search() = %+v, %v", page, err)
	}
	now = now.Add(2 * time.Second)
	if page, err := service.Search(context.Background(), ListFilter{}); err != nil || len(page.Items) != 2 {
		t.Fatalf("expired cached Search() = %+v, %v", page, err)
	}
}

func prepareMigrationTestData(t *testing.T) string {
	t.Helper()
	dataDirectory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDirectory, "migrations"), 0o750); err != nil {
		t.Fatalf("MkdirAll(migrations): %v", err)
	}
	return dataDirectory
}

func writeMigrationTestImage(t *testing.T, dataDirectory, relativePath string, data []byte, modifiedAt time.Time) {
	t.Helper()
	filePath := filepath.Join(dataDirectory, "migrations", filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		t.Fatalf("MkdirAll(%s): %v", relativePath, err)
	}
	if err := os.WriteFile(filePath, data, 0o640); err != nil {
		t.Fatalf("WriteFile(%s): %v", relativePath, err)
	}
	if err := os.Chtimes(filePath, modifiedAt, modifiedAt); err != nil {
		t.Fatalf("Chtimes(%s): %v", relativePath, err)
	}
}

func migrationTestJPEG(t *testing.T) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, 2, 2))
	value.Set(0, 0, color.RGBA{R: 200, A: 255})
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, value, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode(): %v", err)
	}
	return buffer.Bytes()
}

func migrationTestPNG(t *testing.T) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			value.Set(x, y, color.RGBA{R: uint8(x * 12), G: uint8(y * 12), B: uint8((x + y) * 6), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, value); err != nil {
		t.Fatalf("png.Encode(): %v", err)
	}
	return buffer.Bytes()
}
