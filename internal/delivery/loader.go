package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Willxup/imagesilo/internal/platform/storage"
)

type LoadResult struct {
	LoadedIDs  []string
	MissingIDs []string
}

func Load(ctx context.Context, db *sql.DB, filesystem *storage.Filesystem, index *Index) (LoadResult, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, storage_key, mime_type, stored_sha256, stored_size, created_at, visibility, original_name
		FROM images ORDER BY id`)
	if err != nil {
		return LoadResult{}, fmt.Errorf("load delivery targets: %w", err)
	}
	defer rows.Close()

	targets := make(map[string]Target)
	result := LoadResult{}
	for rows.Next() {
		var id string
		var target Target
		var hash []byte
		var createdAt int64
		if err := rows.Scan(
			&id,
			&target.StorageKey,
			&target.MIMEType,
			&hash,
			&target.Size,
			&createdAt,
			&target.Visibility,
			&target.OriginalName,
		); err != nil {
			return LoadResult{}, fmt.Errorf("scan delivery target: %w", err)
		}
		if len(hash) != 32 {
			return LoadResult{}, fmt.Errorf("image %s has invalid stored SHA-256", id)
		}
		exists, err := filesystem.Exists(target.StorageKey)
		if err != nil {
			return LoadResult{}, fmt.Errorf("validate image %s storage: %w", id, err)
		}
		if !exists {
			result.MissingIDs = append(result.MissingIDs, id)
			continue
		}
		target.ETag = fmt.Sprintf("\"%x\"", hash)
		target.LastModified = time.Unix(createdAt, 0).UTC()
		targets[id] = target
		result.LoadedIDs = append(result.LoadedIDs, id)
	}
	if err := rows.Err(); err != nil {
		return LoadResult{}, fmt.Errorf("iterate delivery targets: %w", err)
	}
	index.Replace(targets)
	return result, nil
}
