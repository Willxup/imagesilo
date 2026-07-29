package settings

import (
	"context"
	"time"

	"github.com/Willxup/imagesilo/internal/image"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Get(ctx context.Context) (Settings, error) {
	return s.repository.Get(ctx)
}

func (s *Service) UpdateDefaultVisibility(ctx context.Context, visibility image.Visibility, now time.Time) error {
	if visibility != image.VisibilityPublic && visibility != image.VisibilityPrivate {
		return ErrInvalidVisibility
	}
	return s.repository.UpdateDefaultVisibility(ctx, visibility, now)
}

func (s *Service) UpdateProcessing(ctx context.Context, value Settings, now time.Time) error {
	if value.JPEGQuality < 1 || value.JPEGQuality > 100 ||
		value.WebPQuality < 1 || value.WebPQuality > 100 ||
		value.PNGCompressionLevel < 0 || value.PNGCompressionLevel > 9 ||
		value.ConversionWebPQuality < 1 || value.ConversionWebPQuality > 100 {
		return ErrInvalidProcessing
	}
	return s.repository.UpdateProcessing(ctx, value, now)
}
