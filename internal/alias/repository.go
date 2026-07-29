package imagealias

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mattn/go-sqlite3"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, value Alias) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO image_aliases(id, alias_path, image_id, source, created_at)
		SELECT ?, ?, id, ?, ? FROM images WHERE id = ?`,
		value.ID, value.Path, value.Source, value.CreatedAt.Unix(), value.ImageID,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrAliasConflict
		}
		return fmt.Errorf("create image alias: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count created image aliases: %w", err)
	}
	if affected != 1 {
		return ErrImageNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context, limit int) ([]Alias, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, alias_path, image_id, source, created_at
		FROM image_aliases ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list image aliases: %w", err)
	}
	defer rows.Close()
	result := make([]Alias, 0)
	for rows.Next() {
		value, err := scanAlias(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate image aliases: %w", err)
	}
	return result, nil
}

func (r *Repository) ListByImage(ctx context.Context, imageID string) ([]Alias, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, alias_path, image_id, source, created_at
		FROM image_aliases WHERE image_id = ? ORDER BY created_at ASC, id ASC`, imageID)
	if err != nil {
		return nil, fmt.Errorf("list image aliases by target: %w", err)
	}
	defer rows.Close()
	result := make([]Alias, 0)
	for rows.Next() {
		value, err := scanAlias(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate image aliases by target: %w", err)
	}
	return result, nil
}

func (r *Repository) GetByPath(ctx context.Context, path string) (Alias, error) {
	value, err := scanAlias(r.db.QueryRowContext(ctx, `
		SELECT id, alias_path, image_id, source, created_at
		FROM image_aliases WHERE alias_path = ?`, path))
	if errors.Is(err, sql.ErrNoRows) {
		return Alias{}, ErrAliasNotFound
	}
	return value, err
}

func (r *Repository) Delete(ctx context.Context, id string) (string, error) {
	var path string
	err := r.db.QueryRowContext(ctx, `
		DELETE FROM image_aliases WHERE id = ? RETURNING alias_path`, id).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAliasNotFound
	}
	if err != nil {
		return "", fmt.Errorf("delete image alias: %w", err)
	}
	return path, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAlias(row rowScanner) (Alias, error) {
	var value Alias
	var createdAt int64
	if err := row.Scan(&value.ID, &value.Path, &value.ImageID, &value.Source, &createdAt); err != nil {
		return Alias{}, err
	}
	value.CreatedAt = time.Unix(createdAt, 0).UTC()
	return value, nil
}

func isUniqueConstraint(err error) bool {
	var sqliteError sqlite3.Error
	return errors.As(err, &sqliteError) &&
		(sqliteError.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteError.ExtendedCode == sqlite3.ErrConstraintPrimaryKey)
}
