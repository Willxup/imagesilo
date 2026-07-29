package database

import (
	"context"
	"database/sql"
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
