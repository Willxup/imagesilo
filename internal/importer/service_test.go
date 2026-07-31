package importer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	imagealias "github.com/Willxup/imagesilo/internal/alias"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

type importReadTracker struct {
	read bool
}

func (r *importReadTracker) Read([]byte) (int, error) {
	r.read = true
	return 0, errors.New("reader must not be consumed")
}

func TestImportCommitsImageAliasAndOriginalBytesTogether(t *testing.T) {
	directory, db, service, index := prepareImportTest(t)
	defer db.Close()
	source := testWebP()
	now := time.Unix(1_800_000_000, 0).UTC()
	result, err := service.Import(context.Background(), bytes.NewReader(source), "legacy.webp", "/legacy/one.webp", Options{
		Visibility: images.VisibilityPublic,
		Limits:     processor.Limits{MaxBytes: 1 << 20, MaxTotalPixels: 100},
	}, now)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	expectedHash := sha256.Sum256(source)
	if result.Image.SourceSHA256 != expectedHash || result.Image.StoredSHA256 != expectedHash || result.Image.UploadedVia != "import" {
		t.Fatalf("imported image = %+v", result.Image)
	}
	if result.Alias.Path != "/legacy/one.webp" || result.Alias.ImageID != result.Image.ID {
		t.Fatalf("imported alias = %+v", result.Alias)
	}
	stored, err := os.ReadFile(filepath.Join(directory, "images", result.Image.StorageKey))
	if err != nil || !bytes.Equal(stored, source) {
		t.Fatalf("stored bytes mismatch: error=%v", err)
	}
	if target, ok := index.GetAlias(result.Alias.Path); !ok || target.StorageKey != result.Image.StorageKey {
		t.Fatalf("alias was not published in delivery index: %+v, %t", target, ok)
	}

	beforeImages := countRows(t, db, "images")
	beforeAliases := countRows(t, db, "image_aliases")
	beforeFiles := countFiles(t, filepath.Join(directory, "images"))
	if _, err := service.Import(context.Background(), bytes.NewReader(source), "duplicate.webp", result.Alias.Path, Options{
		Visibility: images.VisibilityPublic,
		Limits:     processor.Limits{MaxBytes: 1 << 20, MaxTotalPixels: 100},
	}, now.Add(time.Second)); !errors.Is(err, imagealias.ErrAliasConflict) {
		t.Fatalf("duplicate Import() error = %v", err)
	}
	if countRows(t, db, "images") != beforeImages || countRows(t, db, "image_aliases") != beforeAliases ||
		countFiles(t, filepath.Join(directory, "images")) != beforeFiles {
		t.Fatal("alias conflict left an image row, alias row, or formal file")
	}
}

func TestBusyImportRejectsBeforeReadingRequestBytes(t *testing.T) {
	_, db, service, _ := prepareImportTest(t)
	defer db.Close()
	release, ok := service.gate.TryAcquire()
	if !ok {
		t.Fatal("failed to occupy processing gate")
	}
	defer release()
	reader := &importReadTracker{}
	_, err := service.Import(context.Background(), reader, "busy.webp", "/legacy/busy.webp", Options{
		Visibility: images.VisibilityPublic,
		Limits:     processor.Limits{MaxBytes: 1 << 20, MaxTotalPixels: 100},
	}, time.Now())
	if !errors.Is(err, images.ErrProcessingBusy) || reader.read {
		t.Fatalf("Import() error = %v, reader consumed = %v", err, reader.read)
	}
}

func prepareImportTest(t *testing.T) (string, *sql.DB, *Service, *delivery.Index) {
	t.Helper()
	directory := t.TempDir()
	for _, path := range []string{"db", "images", filepath.Join("cache", "thumbnails"), "tmp"} {
		if err := os.MkdirAll(filepath.Join(directory, path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", path, err)
		}
	}
	db, err := database.Open(filepath.Join(directory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrations.Apply(): %v", err)
	}
	index := delivery.NewIndex()
	service := NewService(NewRepository(db), storage.NewFilesystem(directory), index, processor.NewEngine(), processor.NewGate(1), indexbarrier.New())
	return directory, db, service, index
}

func testWebP() []byte {
	return []byte{
		0x52, 0x49, 0x46, 0x46, 0x22, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50,
		0x56, 0x50, 0x38, 0x20, 0x16, 0x00, 0x00, 0x00, 0x30, 0x01, 0x00, 0x9d,
		0x01, 0x2a, 0x01, 0x00, 0x01, 0x00, 0x0e, 0xc0, 0xfe, 0x25, 0xa4, 0x00,
		0x03, 0x70, 0x00, 0x00, 0x00, 0x00,
	}
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func countFiles(t *testing.T, directory string) int {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", directory, err)
	}
	return len(entries)
}
