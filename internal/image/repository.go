package image

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, image Image) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO images(
			id, original_name, storage_key, extension, mime_type, width, height,
			source_size, stored_size, source_sha256, stored_sha256, processing_summary,
			visibility, uploaded_via, uploaded_by_api_token_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		image.ID,
		image.OriginalName,
		image.StorageKey,
		image.Extension,
		image.MIMEType,
		image.Width,
		image.Height,
		image.SourceSize,
		image.StoredSize,
		image.SourceSHA256[:],
		image.StoredSHA256[:],
		image.ProcessingSummary,
		image.Visibility,
		image.UploadedVia,
		image.UploadedByAPITokenID,
		image.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create image record: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, limit int) ([]Image, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, original_name, storage_key, extension, mime_type, width, height,
		       source_size, stored_size, source_sha256, stored_sha256, processing_summary,
		       visibility, uploaded_via, uploaded_by_api_token_id, created_at
		FROM images ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	var images []Image
	for rows.Next() {
		value, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate images: %w", err)
	}
	return images, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanImage(row rowScanner) (Image, error) {
	var value Image
	var sourceHash, storedHash []byte
	var createdAt int64
	if err := row.Scan(
		&value.ID,
		&value.OriginalName,
		&value.StorageKey,
		&value.Extension,
		&value.MIMEType,
		&value.Width,
		&value.Height,
		&value.SourceSize,
		&value.StoredSize,
		&sourceHash,
		&storedHash,
		&value.ProcessingSummary,
		&value.Visibility,
		&value.UploadedVia,
		&value.UploadedByAPITokenID,
		&createdAt,
	); err != nil {
		return Image{}, fmt.Errorf("scan image: %w", err)
	}
	if len(sourceHash) != 32 || len(storedHash) != 32 {
		return Image{}, fmt.Errorf("image %s has invalid SHA-256 length", value.ID)
	}
	copy(value.SourceSHA256[:], sourceHash)
	copy(value.StoredSHA256[:], storedHash)
	value.CreatedAt = time.Unix(createdAt, 0).UTC()
	return value, nil
}
