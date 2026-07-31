package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	maxOpenConnections = 4
	maxIdleConnections = 4
)

func Open(path string) (*sql.DB, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := os.Chmod(filepath.Dir(absPath), 0o700); err != nil {
		return nil, fmt.Errorf("secure database directory: %w", err)
	}

	dsn := (&url.URL{
		Scheme:   "file",
		Path:     absPath,
		RawQuery: "_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL&_synchronous=FULL&_txlock=immediate",
	}).String()

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConnections)
	db.SetMaxIdleConns(maxIdleConnections)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := secureSQLiteFiles(absPath); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func secureSQLiteFiles(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(candidate, 0o600); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("secure SQLite file %q: %w", candidate, err)
		}
	}
	return nil
}
