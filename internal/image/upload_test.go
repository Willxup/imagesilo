package image

import (
	"bytes"
	"context"
	"crypto/sha256"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestUploadJPEGPreservesBytesAndUpdatesIndex(t *testing.T) {
	dataDirectory := prepareUploadTestData(t)
	db, err := database.Open(filepath.Join(dataDirectory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}

	jpegBytes := testJPEG(t)
	index := delivery.NewIndex()
	filesystem := storage.NewFilesystem(dataDirectory)
	service := NewService(NewRepository(db), filesystem, index)
	value, err := service.UploadJPEG(context.Background(), bytes.NewReader(jpegBytes), "sample.jpg", time.Now())
	if err != nil {
		t.Fatalf("UploadJPEG() error = %v", err)
	}
	wantHash := sha256.Sum256(jpegBytes)
	if value.SourceSHA256 != wantHash || value.StoredSHA256 != wantHash {
		t.Fatal("UploadJPEG() changed the default-upload bytes")
	}
	stored, err := os.ReadFile(filepath.Join(dataDirectory, "images", value.StorageKey))
	if err != nil {
		t.Fatalf("ReadFile(stored image) error = %v", err)
	}
	if !bytes.Equal(stored, jpegBytes) {
		t.Fatal("stored JPEG does not equal uploaded bytes")
	}
	if target, ok := index.Get(value.ID); !ok || target.StorageKey != value.StorageKey {
		t.Fatal("delivery index was not updated after database commit")
	}

	reloaded := delivery.NewIndex()
	if _, err := delivery.Load(context.Background(), db, filesystem, reloaded); err != nil {
		t.Fatalf("delivery.Load() error = %v", err)
	}
	if _, ok := reloaded.Get(value.ID); !ok {
		t.Fatal("delivery index did not recover after reload")
	}
}

func prepareUploadTestData(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	for _, path := range []string{"db", "images", filepath.Join("cache", "thumbnails"), "tmp"} {
		if err := os.MkdirAll(filepath.Join(directory, path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", path, err)
		}
	}
	return directory
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	value := stdimage.NewRGBA(stdimage.Rect(0, 0, 3, 2))
	value.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, value, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buffer.Bytes()
}
