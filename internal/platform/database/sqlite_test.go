package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAppliesConnectionPragmas(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	connections := make([]*sql.Conn, 0, maxOpenConnections)
	defer func() {
		for _, connection := range connections {
			connection.Close()
		}
	}()
	for i := 0; i < maxOpenConnections; i++ {
		conn, err := db.Conn(context.Background())
		if err != nil {
			t.Fatalf("Conn() error = %v", err)
		}
		connections = append(connections, conn)
		var foreignKeys int
		if err := conn.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("query foreign_keys: %v", err)
		}
		if foreignKeys != 1 {
			t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
		}
		var busyTimeout, synchronous int
		var journalMode string
		if err := conn.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("query busy_timeout: %v", err)
		}
		if err := conn.QueryRowContext(context.Background(), "PRAGMA synchronous").Scan(&synchronous); err != nil {
			t.Fatalf("query synchronous: %v", err)
		}
		if err := conn.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatalf("query journal_mode: %v", err)
		}
		if busyTimeout != 5000 || synchronous != 2 || journalMode != "wal" {
			t.Fatalf("connection pragmas = busy_timeout:%d synchronous:%d journal_mode:%s", busyTimeout, synchronous, journalMode)
		}
	}
}

func TestOpenRestrictsDatabasePermissions(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "imagesilo.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if permission := directoryInfo.Mode().Perm(); permission != 0o700 {
		t.Fatalf("database directory permission = %o, want 700", permission)
	}
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if permission := info.Mode().Perm(); permission != 0o600 {
			t.Fatalf("%s permission = %o, want 600", filepath.Base(candidate), permission)
		}
	}
}
