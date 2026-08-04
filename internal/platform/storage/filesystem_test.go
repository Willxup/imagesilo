package storage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestCommitTemporaryIsAtomicAndRejectsTraversal(t *testing.T) {
	dataDirectory := t.TempDir()
	for _, directory := range []string{"images", "tmp"} {
		if err := os.Mkdir(filepath.Join(dataDirectory, directory), 0o750); err != nil {
			t.Fatalf("Mkdir(%s): %v", directory, err)
		}
	}
	filesystem := NewFilesystem(dataDirectory)
	temporary, err := filesystem.CreateTemporary()
	if err != nil {
		t.Fatalf("CreateTemporary() error = %v", err)
	}
	if _, err := temporary.WriteString("image"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := temporary.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := filesystem.CommitTemporary(temporary.Name(), "safe-id"); err != nil {
		t.Fatalf("CommitTemporary() error = %v", err)
	}
	if _, err := filesystem.Open("../escape"); err == nil {
		t.Fatal("Open() unexpectedly accepted path traversal")
	}
}

func TestManagedOpensRejectSymlinkEscapesAndNonRegularFiles(t *testing.T) {
	dataDirectory := t.TempDir()
	for _, directory := range []string{"images", filepath.Join("cache", "thumbnails")} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, directory), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", directory, err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dataDirectory, "images", "escaped")); err != nil {
		t.Skipf("Symlink unavailable: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dataDirectory, "cache", "thumbnails", "directory"), 0o750); err != nil {
		t.Fatalf("Mkdir(thumbnail): %v", err)
	}
	filesystem := NewFilesystem(dataDirectory)
	if _, err := filesystem.Open("escaped"); err == nil {
		t.Fatal("Open() followed a symlink outside the managed image root")
	}
	if _, err := filesystem.OpenThumbnail("directory"); err == nil {
		t.Fatal("OpenThumbnail() accepted a non-regular path")
	}
}

func TestListMigrationImagesSkipsInvalidContentAndSymlinks(t *testing.T) {
	dataDirectory := t.TempDir()
	migrationsDirectory := filepath.Join(dataDirectory, "migrations")
	if err := os.MkdirAll(filepath.Join(migrationsDirectory, "i", "2026"), 0o750); err != nil {
		t.Fatalf("MkdirAll(migrations): %v", err)
	}
	jpegBytes := storageTestJPEG(t)
	validPath := filepath.Join(migrationsDirectory, "i", "2026", "valid.jpg")
	if err := os.WriteFile(validPath, jpegBytes, 0o640); err != nil {
		t.Fatalf("WriteFile(valid): %v", err)
	}
	disguisedBytes := []byte("not an image")
	if err := os.WriteFile(filepath.Join(migrationsDirectory, "disguised.jpg"), disguisedBytes, 0o640); err != nil {
		t.Fatalf("WriteFile(disguised): %v", err)
	}
	notesBytes := []byte("not public")
	if err := os.WriteFile(filepath.Join(migrationsDirectory, "notes.txt"), notesBytes, 0o640); err != nil {
		t.Fatalf("WriteFile(notes): %v", err)
	}
	if err := os.Symlink(validPath, filepath.Join(migrationsDirectory, "linked.jpg")); err != nil {
		t.Logf("Symlink unavailable: %v", err)
	}

	listed, err := NewFilesystem(dataDirectory).ListMigrationImages(context.Background())
	if err != nil {
		t.Fatalf("ListMigrationImages() error = %v", err)
	}
	if len(listed.Files) != 1 || listed.Files[0].RelativePath != "i/2026/valid.jpg" || listed.Files[0].MIMEType != "image/jpeg" {
		t.Fatalf("ListMigrationImages() = %+v", listed)
	}
	if listed.SkippedFiles < 2 {
		t.Fatalf("SkippedFiles = %d, want at least invalid content and unsupported extension", listed.SkippedFiles)
	}
	if want := int64(len(jpegBytes) + len(disguisedBytes) + len(notesBytes)); listed.StoredBytes != want {
		t.Fatalf("StoredBytes = %d, want %d for all regular files", listed.StoredBytes, want)
	}
}

func TestListMigrationImagesHonorsCancellation(t *testing.T) {
	dataDirectory := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDirectory, "migrations"), 0o750); err != nil {
		t.Fatalf("Mkdir(migrations): %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewFilesystem(dataDirectory).ListMigrationImages(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListMigrationImages() error = %v, want context cancellation", err)
	}
}

func TestRemoveMigrationImageCleansOnlyEmptyAncestors(t *testing.T) {
	dataDirectory := t.TempDir()
	migrationsDirectory := filepath.Join(dataDirectory, "migrations")
	deleteDirectory := filepath.Join(migrationsDirectory, "i", "2022", "04")
	keepDirectory := filepath.Join(migrationsDirectory, "i", "2023")
	for _, directory := range []string{deleteDirectory, keepDirectory} {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", directory, err)
		}
	}
	jpegBytes := storageTestJPEG(t)
	deletePath := filepath.Join(deleteDirectory, "delete.jpg")
	keepPath := filepath.Join(keepDirectory, "keep.jpg")
	for _, path := range []string{deletePath, keepPath} {
		if err := os.WriteFile(path, jpegBytes, 0o640); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	result, err := NewFilesystem(dataDirectory).RemoveMigrationImage("i/2022/04/delete.jpg")
	if err != nil {
		t.Fatalf("RemoveMigrationImage() error = %v", err)
	}
	if result.RemovedDirectories != 2 || result.DirectoryCleanupError != nil {
		t.Fatalf("RemoveMigrationImage() = %+v, want two empty directories removed", result)
	}
	if _, err := os.Stat(deletePath); !os.IsNotExist(err) {
		t.Fatalf("deleted migration image stat error = %v", err)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("sibling migration image was affected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(migrationsDirectory, "i")); err != nil {
		t.Fatalf("non-empty ancestor was removed: %v", err)
	}
	if _, err := NewFilesystem(dataDirectory).RemoveMigrationImage("../outside.jpg"); err == nil {
		t.Fatal("RemoveMigrationImage() accepted traversal")
	}
}

func TestRemoveMigrationImageRejectsIntermediateDirectorySymlink(t *testing.T) {
	dataDirectory := t.TempDir()
	migrationsDirectory := filepath.Join(dataDirectory, "migrations")
	actualDirectory := filepath.Join(migrationsDirectory, "actual")
	if err := os.MkdirAll(actualDirectory, 0o750); err != nil {
		t.Fatalf("MkdirAll(actual): %v", err)
	}
	for _, name := range []string{"delete.jpg", "keep.jpg"} {
		if err := os.WriteFile(filepath.Join(actualDirectory, name), storageTestJPEG(t), 0o640); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	symlinkPath := filepath.Join(migrationsDirectory, "i")
	if err := os.Symlink("actual", symlinkPath); err != nil {
		t.Skipf("Symlink unavailable: %v", err)
	}

	_, err := NewFilesystem(dataDirectory).RemoveMigrationImage("i/delete.jpg")
	if !errors.Is(err, ErrUnsafeMigrationPath) {
		t.Fatalf("RemoveMigrationImage() error = %v, want ErrUnsafeMigrationPath", err)
	}
	if info, err := os.Lstat(symlinkPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("intermediate symlink changed: info = %v, error = %v", info, err)
	}
	for _, name := range []string{"delete.jpg", "keep.jpg"} {
		if _, err := os.Stat(filepath.Join(actualDirectory, name)); err != nil {
			t.Fatalf("target file %s was affected: %v", name, err)
		}
	}
}

func storageTestJPEG(t *testing.T) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, 2, 2))
	value.Set(0, 0, color.RGBA{R: 180, A: 255})
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, value, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode(): %v", err)
	}
	return buffer.Bytes()
}
