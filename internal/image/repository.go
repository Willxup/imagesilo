package image

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	return r.ListFiltered(ctx, repositoryListFilter{Limit: limit})
}

type repositoryListFilter struct {
	Limit          int
	BeforeUnix     int64
	BeforeID       string
	Query          string
	Visibility     Visibility
	MIMEType       string
	UploadedVia    string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	MinStoredBytes int64
	MaxStoredBytes int64
	MinWidth       int
	MaxWidth       int
	MinHeight      int
	MaxHeight      int
}

func (r *Repository) ListFiltered(ctx context.Context, filter repositoryListFilter) ([]Image, error) {
	var query strings.Builder
	query.WriteString(`
		SELECT id, original_name, storage_key, extension, mime_type, width, height,
		       source_size, stored_size, source_sha256, stored_sha256, processing_summary,
		       visibility, uploaded_via, uploaded_by_api_token_id, created_at
		FROM images WHERE 1 = 1`)
	args := make([]any, 0, 20)
	if filter.BeforeID != "" {
		query.WriteString(" AND (created_at < ? OR (created_at = ? AND id < ?))")
		args = append(args, filter.BeforeUnix, filter.BeforeUnix, filter.BeforeID)
	}
	if filter.Query != "" {
		pattern := "%" + escapeLike(strings.ToLower(filter.Query)) + "%"
		query.WriteString(` AND (
			LOWER(original_name) LIKE ? ESCAPE '\' OR LOWER(id) LIKE ? ESCAPE '\' OR
			LOWER(HEX(source_sha256)) LIKE ? ESCAPE '\' OR LOWER(HEX(stored_sha256)) LIKE ? ESCAPE '\' OR
			EXISTS (SELECT 1 FROM image_aliases WHERE image_aliases.image_id = images.id AND LOWER(alias_path) LIKE ? ESCAPE '\')
		)`)
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}
	if filter.Visibility != "" {
		query.WriteString(" AND visibility = ?")
		args = append(args, filter.Visibility)
	}
	if filter.MIMEType != "" {
		query.WriteString(" AND mime_type = ?")
		args = append(args, filter.MIMEType)
	}
	if filter.UploadedVia != "" {
		query.WriteString(" AND uploaded_via = ?")
		args = append(args, filter.UploadedVia)
	}
	if filter.CreatedFrom != nil {
		query.WriteString(" AND created_at >= ?")
		args = append(args, filter.CreatedFrom.Unix())
	}
	if filter.CreatedTo != nil {
		query.WriteString(" AND created_at <= ?")
		args = append(args, filter.CreatedTo.Unix())
	}
	appendIntegerFilter(&query, &args, "stored_size", filter.MinStoredBytes, filter.MaxStoredBytes)
	appendIntegerFilter(&query, &args, "width", int64(filter.MinWidth), int64(filter.MaxWidth))
	appendIntegerFilter(&query, &args, "height", int64(filter.MinHeight), int64(filter.MaxHeight))
	query.WriteString(" ORDER BY created_at DESC, id DESC LIMIT ?")
	args = append(args, filter.Limit)

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
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

func (r *Repository) Get(ctx context.Context, id string) (Image, error) {
	value, err := scanImage(r.db.QueryRowContext(ctx, `
		SELECT id, original_name, storage_key, extension, mime_type, width, height,
		       source_size, stored_size, source_sha256, stored_sha256, processing_summary,
		       visibility, uploaded_via, uploaded_by_api_token_id, created_at
		FROM images WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrImageNotFound
	}
	return value, err
}

func (r *Repository) Delete(ctx context.Context, id string) (Image, error) {
	value, err := scanImage(r.db.QueryRowContext(ctx, `
		DELETE FROM images WHERE id = ?
		RETURNING id, original_name, storage_key, extension, mime_type, width, height,
		          source_size, stored_size, source_sha256, stored_sha256, processing_summary,
		          visibility, uploaded_via, uploaded_by_api_token_id, created_at`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrImageNotFound
	}
	return value, err
}

func (r *Repository) UpdateVisibility(ctx context.Context, id string, visibility Visibility) (bool, error) {
	result, err := r.db.ExecContext(ctx, "UPDATE images SET visibility = ? WHERE id = ?", visibility, id)
	if err != nil {
		return false, fmt.Errorf("update image visibility: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count image visibility update: %w", err)
	}
	return updated == 1, nil
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

func appendIntegerFilter(query *strings.Builder, args *[]any, column string, minimum, maximum int64) {
	if minimum > 0 {
		query.WriteString(" AND " + column + " >= ?")
		*args = append(*args, minimum)
	}
	if maximum > 0 {
		query.WriteString(" AND " + column + " <= ?")
		*args = append(*args, maximum)
	}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}
