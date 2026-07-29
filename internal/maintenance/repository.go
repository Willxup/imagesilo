package maintenance

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository struct {
	db *sql.DB
}

type PersistentStats struct {
	ImageCount     int64
	AliasCount     int64
	StoredBytes    int64
	ActiveSessions int64
	ActiveTokens   int64
}

type ImageFileRecord struct {
	ID         string
	StorageKey string
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Stats(ctx context.Context, now time.Time) (PersistentStats, error) {
	var result PersistentStats
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(stored_size), 0) FROM images`).Scan(&result.ImageCount, &result.StoredBytes); err != nil {
		return PersistentStats{}, fmt.Errorf("read image storage statistics: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM image_aliases").Scan(&result.AliasCount); err != nil {
		return PersistentStats{}, fmt.Errorf("read alias statistics: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE expires_at > ?", now.Unix()).Scan(&result.ActiveSessions); err != nil {
		return PersistentStats{}, fmt.Errorf("read active session statistics: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM api_tokens
		WHERE status = 'active' AND (expires_at IS NULL OR expires_at > ?)`, now.Unix()).Scan(&result.ActiveTokens); err != nil {
		return PersistentStats{}, fmt.Errorf("read active API token statistics: %w", err)
	}
	return result, nil
}

func (r *Repository) ImageFiles(ctx context.Context) ([]ImageFileRecord, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, storage_key FROM images ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list image file records: %w", err)
	}
	defer rows.Close()
	result := make([]ImageFileRecord, 0)
	for rows.Next() {
		var value ImageFileRecord
		if err := rows.Scan(&value.ID, &value.StorageKey); err != nil {
			return nil, fmt.Errorf("scan image file record: %w", err)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate image file records: %w", err)
	}
	return result, nil
}
