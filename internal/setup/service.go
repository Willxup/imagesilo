package setup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/Willxup/imagesilo/internal/auth"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/google/uuid"
)

type Request struct {
	BootstrapToken        string
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
	db                 *sql.DB
	initializeMu       sync.Mutex
	bootstrapTokenHash [32]byte
	bootstrapTokenSet  bool
	hashPassword       func(string) (string, error)
}

func NewService(ctx context.Context, db *sql.DB) (*Service, string, error) {
	service := &Service{db: db, hashPassword: auth.HashPassword}
	initialized, err := service.Initialized(ctx)
	if err != nil {
		return nil, "", err
	}
	if initialized {
		return service, "", nil
	}
	token, hash, err := newBootstrapToken()
	if err != nil {
		return nil, "", err
	}
	service.bootstrapTokenHash = hash
	service.bootstrapTokenSet = true
	return service, token, nil
}

func (s *Service) Initialized(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin").Scan(&count); err != nil {
		return false, fmt.Errorf("count administrators: %w", err)
	}
	return count > 0, nil
}

func (s *Service) Initialize(ctx context.Context, request Request, now time.Time) (auth.Admin, error) {
	s.initializeMu.Lock()
	defer s.initializeMu.Unlock()

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin").Scan(&count); err != nil {
		return auth.Admin{}, fmt.Errorf("check initial setup: %w", err)
	}
	if count > 0 {
		return auth.Admin{}, ErrAlreadyInitialized
	}
	providedHash := sha256.Sum256([]byte(request.BootstrapToken))
	if !s.bootstrapTokenSet || subtle.ConstantTimeCompare(providedHash[:], s.bootstrapTokenHash[:]) != 1 {
		return auth.Admin{}, ErrInvalidBootstrapToken
	}

	displayName, err := auth.NormalizeDisplayName(request.DisplayName)
	if err != nil {
		return auth.Admin{}, err
	}
	email, err := auth.NormalizeEmail(request.Email)
	if err != nil {
		return auth.Admin{}, err
	}
	passwordHash, err := s.hashPassword(request.Password)
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
	count = 0
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
	s.bootstrapTokenHash = [32]byte{}
	s.bootstrapTokenSet = false
	return admin, nil
}

func newBootstrapToken() (string, [32]byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate bootstrap token: %w", err)
	}
	token := "isb_" + base64.RawURLEncoding.EncodeToString(raw)
	return token, sha256.Sum256([]byte(token)), nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
