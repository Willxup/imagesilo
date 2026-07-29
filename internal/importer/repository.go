package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	imagealias "github.com/Willxup/imagesilo/internal/alias"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/mattn/go-sqlite3"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) AliasExists(ctx context.Context, path string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM image_aliases WHERE alias_path = ?)", path).Scan(&exists); err != nil {
		return false, fmt.Errorf("check imported alias: %w", err)
	}
	return exists, nil
}

func (r *Repository) Create(ctx context.Context, image images.Image, alias imagealias.Alias) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin image import: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO images(
			id, original_name, storage_key, extension, mime_type, width, height,
			source_size, stored_size, source_sha256, stored_sha256, processing_summary,
			visibility, uploaded_via, uploaded_by_api_token_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'import', ?, ?)`,
		image.ID, image.OriginalName, image.StorageKey, image.Extension, image.MIMEType, image.Width, image.Height,
		image.SourceSize, image.StoredSize, image.SourceSHA256[:], image.StoredSHA256[:], image.ProcessingSummary,
		image.Visibility, image.UploadedByAPITokenID, image.CreatedAt.Unix(),
	); err != nil {
		return fmt.Errorf("create imported image record: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO image_aliases(id, alias_path, image_id, source, created_at)
		VALUES (?, ?, ?, ?, ?)`, alias.ID, alias.Path, alias.ImageID, alias.Source, alias.CreatedAt.Unix()); err != nil {
		if isAliasConflict(err) {
			return imagealias.ErrAliasConflict
		}
		return fmt.Errorf("create imported image alias: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit image import: %w", err)
	}
	return nil
}

func isAliasConflict(err error) bool {
	var sqliteError sqlite3.Error
	return errors.As(err, &sqliteError) &&
		(sqliteError.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteError.ExtendedCode == sqlite3.ErrConstraintPrimaryKey)
}
