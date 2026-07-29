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
