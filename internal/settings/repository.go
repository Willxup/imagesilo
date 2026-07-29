package settings

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Willxup/imagesilo/internal/image"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Get(ctx context.Context) (Settings, error) {
	var value Settings
	var compressionEnabled, conversionEnabled, conversionLossless int
	err := r.db.QueryRowContext(ctx, `
		SELECT max_upload_bytes, max_batch_count, max_total_pixels,
		       compression_enabled, jpeg_quality, webp_quality, png_compression_level,
		       conversion_enabled, conversion_webp_quality, conversion_webp_lossless,
		       default_visibility, maintenance_hour
		FROM app_settings WHERE singleton = 1`).Scan(
		&value.MaxUploadBytes,
		&value.MaxBatchCount,
		&value.MaxTotalPixels,
		&compressionEnabled,
		&value.JPEGQuality,
		&value.WebPQuality,
		&value.PNGCompressionLevel,
		&conversionEnabled,
		&value.ConversionWebPQuality,
		&conversionLossless,
		&value.DefaultVisibility,
		&value.MaintenanceHour,
	)
	if err != nil {
		return Settings{}, fmt.Errorf("read application settings: %w", err)
	}
	value.CompressionEnabled = compressionEnabled == 1
	value.ConversionEnabled = conversionEnabled == 1
	value.ConversionWebPLossless = conversionLossless == 1
	return value, nil
}

func (r *Repository) UpdateDefaultVisibility(ctx context.Context, visibility image.Visibility, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE app_settings SET default_visibility = ?, updated_at = ? WHERE singleton = 1`,
		visibility, now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("update default visibility: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return fmt.Errorf("default visibility update affected %d rows", updated)
	}
	return nil
}

func (r *Repository) UpdateProcessing(ctx context.Context, value Settings, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE app_settings SET
			compression_enabled = ?, jpeg_quality = ?, webp_quality = ?, png_compression_level = ?,
			conversion_enabled = ?, conversion_webp_quality = ?, conversion_webp_lossless = ?, updated_at = ?
		WHERE singleton = 1`,
		boolInt(value.CompressionEnabled), value.JPEGQuality, value.WebPQuality, value.PNGCompressionLevel,
		boolInt(value.ConversionEnabled), value.ConversionWebPQuality, boolInt(value.ConversionWebPLossless), now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("update image processing settings: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return fmt.Errorf("image processing settings update affected %d rows", updated)
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
