package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Willxup/imagesilo/internal/platform/storage"
)

type LoadResult struct {
	LoadedIDs         []string
	MissingIDs        []string
	LoadedAliasCount  int
	SkippedAliasCount int
}

type Snapshot struct {
	Targets map[string]Target
	Aliases map[string]string
}

func Build(ctx context.Context, db *sql.DB, filesystem *storage.Filesystem) (Snapshot, LoadResult, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, storage_key, mime_type, stored_sha256, stored_size, created_at, visibility, original_name
		FROM images ORDER BY id`)
	if err != nil {
		return Snapshot{}, LoadResult{}, fmt.Errorf("load delivery targets: %w", err)
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
			return Snapshot{}, LoadResult{}, fmt.Errorf("scan delivery target: %w", err)
		}
		if len(hash) != 32 {
			return Snapshot{}, LoadResult{}, fmt.Errorf("image %s has invalid stored SHA-256", id)
		}
		exists, err := filesystem.Exists(target.StorageKey)
		if err != nil {
			return Snapshot{}, LoadResult{}, fmt.Errorf("validate image %s storage: %w", id, err)
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
		return Snapshot{}, LoadResult{}, fmt.Errorf("iterate delivery targets: %w", err)
	}
	if err := rows.Close(); err != nil {
		return Snapshot{}, LoadResult{}, fmt.Errorf("close delivery target rows: %w", err)
	}

	aliasRows, err := db.QueryContext(ctx, `
		SELECT alias_path, image_id FROM image_aliases ORDER BY alias_path`)
	if err != nil {
		return Snapshot{}, LoadResult{}, fmt.Errorf("load delivery aliases: %w", err)
	}
	defer aliasRows.Close()
	aliases := make(map[string]string)
	missing := make(map[string]struct{}, len(result.MissingIDs))
	for _, id := range result.MissingIDs {
		missing[id] = struct{}{}
	}
	for aliasRows.Next() {
		var aliasPath, imageID string
		if err := aliasRows.Scan(&aliasPath, &imageID); err != nil {
			return Snapshot{}, LoadResult{}, fmt.Errorf("scan delivery alias: %w", err)
		}
		normalized, err := NormalizeAliasPath(aliasPath)
		if err != nil {
			return Snapshot{}, LoadResult{}, fmt.Errorf("validate alias %q: %w", aliasPath, err)
		}
		if normalized != aliasPath {
			return Snapshot{}, LoadResult{}, fmt.Errorf("alias %q is not stored in canonical form", aliasPath)
		}
		if _, ok := missing[imageID]; ok {
			result.SkippedAliasCount++
			continue
		}
		if _, ok := targets[imageID]; !ok {
			return Snapshot{}, LoadResult{}, fmt.Errorf("alias %q references unknown image %s", aliasPath, imageID)
		}
		if _, exists := aliases[aliasPath]; exists {
			return Snapshot{}, LoadResult{}, fmt.Errorf("duplicate alias path %q", aliasPath)
		}
		aliases[aliasPath] = imageID
		result.LoadedAliasCount++
	}
	if err := aliasRows.Err(); err != nil {
		return Snapshot{}, LoadResult{}, fmt.Errorf("iterate delivery aliases: %w", err)
	}
	return Snapshot{Targets: targets, Aliases: aliases}, result, nil
}

func Load(ctx context.Context, db *sql.DB, filesystem *storage.Filesystem, index *Index) (LoadResult, error) {
	snapshot, result, err := Build(ctx, db, filesystem)
	if err != nil {
		return LoadResult{}, err
	}
	index.ReplaceAll(snapshot.Targets, snapshot.Aliases)
	return result, nil
}
