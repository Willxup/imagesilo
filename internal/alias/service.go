package imagealias

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	page, err := s.ListPage(ctx, limit, "")
	return page.Items, err
}

type listCursor struct {
	CreatedAt int64  `json:"createdAt"`
	ID        string `json:"id"`
}

func (s *Service) ListPage(ctx context.Context, limit int, rawCursor string) (Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var cursor listCursor
	if rawCursor != "" {
		if len(rawCursor) > 512 {
			return Page{}, ErrInvalidCursor
		}
		decoded, err := base64.RawURLEncoding.DecodeString(rawCursor)
		if err != nil || json.Unmarshal(decoded, &cursor) != nil || cursor.CreatedAt <= 0 {
			return Page{}, ErrInvalidCursor
		}
		parsed, err := uuid.Parse(cursor.ID)
		if err != nil || parsed.String() != cursor.ID {
			return Page{}, ErrInvalidCursor
		}
	}
	values, err := s.repository.ListPage(ctx, limit+1, cursor.CreatedAt, cursor.ID)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: values}
	if len(values) > limit {
		page.Items = values[:limit]
		last := page.Items[len(page.Items)-1]
		encoded, err := json.Marshal(listCursor{CreatedAt: last.CreatedAt.Unix(), ID: last.ID})
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return page, nil
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
