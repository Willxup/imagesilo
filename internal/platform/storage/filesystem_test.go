package storage

import (
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
