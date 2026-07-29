package imagealias

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/google/uuid"
)

type Service struct {
	repository *Repository
	index      *delivery.Index
	barrier    *indexbarrier.Barrier
}

func NewService(repository *Repository, index *delivery.Index, barrier *indexbarrier.Barrier) *Service {
	return &Service{repository: repository, index: index, barrier: barrier}
}

func (s *Service) Create(ctx context.Context, path, imageID, source string, now time.Time) (Alias, error) {
	normalizedPath, err := delivery.NormalizeAliasPath(path)
	if err != nil {
		return Alias{}, err
	}
	parsedImageID, err := uuid.Parse(imageID)
	if err != nil || parsedImageID.String() != imageID {
		return Alias{}, ErrInvalidImage
	}
	source = strings.TrimSpace(source)
	if source == "" || len([]rune(source)) > 100 || strings.IndexFunc(source, unicode.IsControl) >= 0 {
		return Alias{}, ErrInvalidSource
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Alias{}, fmt.Errorf("generate image alias id: %w", err)
	}
	value := Alias{ID: id.String(), Path: normalizedPath, ImageID: imageID, Source: source, CreatedAt: now.UTC()}
	release := s.barrier.BeginChange()
	defer release()
	if err := s.repository.Create(ctx, value); err != nil {
		return Alias{}, err
	}
	s.index.AddAlias(value.Path, value.ImageID)
	return value, nil
}

func (s *Service) List(ctx context.Context, limit int) ([]Alias, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repository.List(ctx, limit)
}

func (s *Service) ListByImage(ctx context.Context, imageID string) ([]Alias, error) {
	parsedImageID, err := uuid.Parse(imageID)
	if err != nil || parsedImageID.String() != imageID {
		return nil, ErrInvalidImage
	}
	return s.repository.ListByImage(ctx, imageID)
}

func (s *Service) Resolve(ctx context.Context, path string) (Alias, error) {
	normalizedPath, err := delivery.NormalizeAliasPath(path)
	if err != nil {
		return Alias{}, err
	}
	return s.repository.GetByPath(ctx, normalizedPath)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil || parsedID.String() != id {
		return ErrAliasNotFound
	}
	release := s.barrier.BeginChange()
	defer release()
	path, err := s.repository.Delete(ctx, id)
	if err != nil {
		return err
	}
	s.index.RemoveAlias(path)
	return nil
}
