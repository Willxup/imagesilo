package image

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
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

func TestUploadPreservesBytesAndUpdatesIndex(t *testing.T) {
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
	value, err := service.Upload(context.Background(), bytes.NewReader(jpegBytes), "sample.jpg", UploadOptions{
		Visibility:  VisibilityPublic,
		UploadedVia: "admin",
	}, time.Now())
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	wantHash := sha256.Sum256(jpegBytes)
	if value.SourceSHA256 != wantHash || value.StoredSHA256 != wantHash {
		t.Fatal("Upload() changed the default-upload bytes")
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

func TestUploadPublicationSerializesWithVisibilityChange(t *testing.T) {
	_, db, service, index := prepareManagementTest(t)
	defer db.Close()
	committed := make(chan string, 1)
	resume := make(chan struct{})
	service.afterCreateCommit = func(id string) {
		committed <- id
		<-resume
	}
	uploaded := make(chan error, 1)
	source := testJPEG(t)
	go func() {
		_, err := service.Upload(context.Background(), bytes.NewReader(source), "visibility.jpg", UploadOptions{
			Visibility: VisibilityPublic, UploadedVia: "admin",
		}, time.Now())
		uploaded <- err
	}()
	var id string
	select {
	case id = <-committed:
	case err := <-uploaded:
		t.Fatalf("upload stopped before publication: %v", err)
	case <-time.After(5 * time.Second):
		close(resume)
		t.Fatal("upload did not reach publication")
	}
	changed := make(chan error, 1)
	go func() {
		found, err := service.ChangeVisibility(context.Background(), id, VisibilityPrivate)
		if err == nil && !found {
			err = errors.New("committed image was not found")
		}
		changed <- err
	}()
	completedEarly := false
	select {
	case err := <-changed:
		completedEarly = true
		t.Errorf("visibility change passed unfinished upload publication: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(resume)
	if err := <-uploaded; err != nil {
		t.Fatal(err)
	}
	if !completedEarly {
		if err := <-changed; err != nil {
			t.Fatal(err)
		}
	}
	stored, err := service.Get(context.Background(), id)
	if err != nil || stored.Visibility != VisibilityPrivate {
		t.Fatalf("stored image = %+v, %v", stored, err)
	}
	if target, ok := index.Get(id); !ok || target.Visibility != string(VisibilityPrivate) {
		t.Fatalf("private upload remained publicly deliverable: %+v, found=%v", target, ok)
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
