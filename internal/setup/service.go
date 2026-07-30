package setup

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Willxup/imagesilo/internal/auth"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/google/uuid"
)

type Request struct {
	DisplayName           string
	Email                 string
	Password              string
	DefaultVisibility     images.Visibility
	CompressionEnabled    bool
	JPEGQuality           int
	WebPQuality           int
	PNGCompressionLevel   int
	ConversionEnabled     bool
	ConversionWebPQuality int
	ConversionLossless    bool
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Initialized(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin").Scan(&count); err != nil {
		return false, fmt.Errorf("count administrators: %w", err)
	}
	return count > 0, nil
}

func (s *Service) Initialize(ctx context.Context, request Request, now time.Time) (auth.Admin, error) {
	displayName, err := auth.NormalizeDisplayName(request.DisplayName)
	if err != nil {
		return auth.Admin{}, err
	}
	email, err := auth.NormalizeEmail(request.Email)
	if err != nil {
		return auth.Admin{}, err
	}
	passwordHash, err := auth.HashPassword(request.Password)
	if err != nil {
		return auth.Admin{}, err
	}
	if request.DefaultVisibility != images.VisibilityPublic && request.DefaultVisibility != images.VisibilityPrivate ||
		request.JPEGQuality < 1 || request.JPEGQuality > 100 ||
		request.WebPQuality < 1 || request.WebPQuality > 100 ||
		request.PNGCompressionLevel < 0 || request.PNGCompressionLevel > 9 ||
		request.ConversionWebPQuality < 1 || request.ConversionWebPQuality > 100 {
		return auth.Admin{}, ErrInvalidSettings
	}
	id, err := uuid.NewV7()
	if err != nil {
		return auth.Admin{}, fmt.Errorf("generate administrator id: %w", err)
	}
	admin := auth.Admin{
		ID: id.String(), DisplayName: displayName, Email: email, PasswordHash: passwordHash,
		CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.Admin{}, fmt.Errorf("begin initial setup: %w", err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin").Scan(&count); err != nil {
		return auth.Admin{}, fmt.Errorf("check initial setup: %w", err)
	}
	if count > 0 {
		return auth.Admin{}, ErrAlreadyInitialized
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO admin(id, display_name, email, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		admin.ID, admin.DisplayName, admin.Email, admin.PasswordHash, admin.CreatedAt.Unix(), admin.UpdatedAt.Unix(),
	); err != nil {
		return auth.Admin{}, fmt.Errorf("create initial administrator: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE app_settings SET
			default_visibility = ?, compression_enabled = ?, jpeg_quality = ?, webp_quality = ?, png_compression_level = ?,
			conversion_enabled = ?, conversion_webp_quality = ?, conversion_webp_lossless = ?, updated_at = ?
		WHERE singleton = 1`,
		request.DefaultVisibility, boolInt(request.CompressionEnabled), request.JPEGQuality, request.WebPQuality, request.PNGCompressionLevel,
		boolInt(request.ConversionEnabled), request.ConversionWebPQuality, boolInt(request.ConversionLossless), now.Unix(),
	); err != nil {
		return auth.Admin{}, fmt.Errorf("save initial settings: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return auth.Admin{}, fmt.Errorf("commit initial setup: %w", err)
	}
	return admin, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
