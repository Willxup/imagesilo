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
