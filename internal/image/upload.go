package image

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/google/uuid"
)

const (
	maxUploadBytes = 20 << 20
	maxTotalPixels = 16_000_000
)

type Service struct {
	repository *Repository
	storage    *storage.Filesystem
	index      *delivery.Index
	processor  processor.Engine
	gate       *processor.Gate
	barrier    *indexbarrier.Barrier
}

func NewService(repository *Repository, filesystem *storage.Filesystem, index *delivery.Index) *Service {
	return NewServiceWithProcessor(repository, filesystem, index, processor.NewEngine(), processor.NewGate(1))
}

func NewServiceWithProcessor(
	repository *Repository,
	filesystem *storage.Filesystem,
	index *delivery.Index,
	engine processor.Engine,
	gate *processor.Gate,
) *Service {
	return NewServiceWithProcessorAndBarrier(repository, filesystem, index, engine, gate, indexbarrier.New())

}

func NewServiceWithProcessorAndBarrier(
	repository *Repository,
	filesystem *storage.Filesystem,
	index *delivery.Index,
	engine processor.Engine,
	gate *processor.Gate,
	barrier *indexbarrier.Barrier,
) *Service {
	return &Service{repository: repository, storage: filesystem, index: index, processor: engine, gate: gate, barrier: barrier}
}

func (s *Service) Upload(ctx context.Context, reader io.Reader, originalName string, options UploadOptions, now time.Time) (Image, error) {
	if options.Visibility != VisibilityPublic && options.Visibility != VisibilityPrivate {
		return Image{}, fmt.Errorf("invalid image visibility")
	}
	if options.UploadedVia != "admin" && options.UploadedVia != "api_token" && options.UploadedVia != "import" {
		return Image{}, fmt.Errorf("invalid upload source")
	}
	if options.Limits.MaxBytes <= 0 {
		options.Limits.MaxBytes = maxUploadBytes
	}
	if options.Limits.MaxTotalPixels <= 0 {
		options.Limits.MaxTotalPixels = maxTotalPixels
	}

	temporary, err := s.storage.CreateTemporary()
	if err != nil {
		return Image{}, err
	}
	sourcePath := temporary.Name()
	defer s.storage.RemoveTemporary(sourcePath)

	sourceHasher := sha256.New()
	limited := io.LimitReader(reader, options.Limits.MaxBytes+1)
	buffer := make([]byte, 64*1024)
	written, copyErr := io.CopyBuffer(io.MultiWriter(temporary, sourceHasher), limited, buffer)
	closeErr := temporary.Close()
	if copyErr != nil {
		return Image{}, fmt.Errorf("stream upload: %w", copyErr)
	}
	if closeErr != nil {
		return Image{}, fmt.Errorf("close upload temporary file: %w", closeErr)
	}
	if written == 0 {
		return Image{}, ErrInvalidImage
	}
	if written > options.Limits.MaxBytes {
		return Image{}, ErrFileTooLarge
	}
	var sourceDigest [32]byte
	copy(sourceDigest[:], sourceHasher.Sum(nil))

	release, ok := s.gate.TryAcquire()
	if !ok {
		return Image{}, ErrProcessingBusy
	}
	metadata, selectedPath, thumbnailPath, summary, err := func() (processor.Metadata, string, string, string, error) {
		defer release()
		defer processor.TrimMemory()
		return s.prepareImage(ctx, sourcePath, written, options)
	}()
	if err != nil {
		return Image{}, mapProcessorError(err)
	}
	if selectedPath != sourcePath {
		defer s.storage.RemoveTemporary(selectedPath)
	}
	if thumbnailPath != "" {
		defer s.storage.RemoveTemporary(thumbnailPath)
	}

	storedSize := written
	storedDigest := sourceDigest
	if selectedPath != sourcePath {
		storedSize, storedDigest, err = hashFile(selectedPath)
		if err != nil {
			return Image{}, err
		}
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Image{}, fmt.Errorf("generate image id: %w", err)
	}
	storageKey := id.String()
	if _, err := s.storage.CommitTemporary(selectedPath, storageKey); err != nil {
		return Image{}, err
	}
	databaseCommitted := false
	thumbnailCommitted := false
	defer func() {
		if !databaseCommitted {
			_ = s.storage.Remove(storageKey)
			if thumbnailCommitted {
				_ = s.storage.RemoveThumbnail(id.String())
			}
		}
	}()
	if thumbnailPath != "" {
		if err := s.storage.CommitThumbnailTemporary(thumbnailPath, id.String()); err != nil {
			return Image{}, err
		}
		thumbnailCommitted = true
	}

	value := Image{
		ID:                   id.String(),
		OriginalName:         sanitizeOriginalName(originalName),
		StorageKey:           storageKey,
		Extension:            metadata.Extension,
		MIMEType:             metadata.MIMEType,
		Width:                metadata.Width,
		Height:               metadata.Height,
		SourceSize:           written,
		StoredSize:           storedSize,
		SourceSHA256:         sourceDigest,
		StoredSHA256:         storedDigest,
		ProcessingSummary:    summary,
		Visibility:           options.Visibility,
		UploadedVia:          options.UploadedVia,
		UploadedByAPITokenID: options.UploadedByAPITokenID,
		CreatedAt:            now.UTC(),
	}
	releaseChange := s.barrier.BeginChange()
	defer releaseChange()
	if err := s.repository.Create(ctx, value); err != nil {
		return Image{}, err
	}
	databaseCommitted = true
	s.index.Add(value.ID, delivery.Target{
		StorageKey:   value.StorageKey,
		MIMEType:     value.MIMEType,
		ETag:         fmt.Sprintf("\"%x\"", value.StoredSHA256),
		Size:         value.StoredSize,
		LastModified: value.CreatedAt,
		Visibility:   string(value.Visibility),
		OriginalName: value.OriginalName,
	})
	return value, nil
}

type processingSummary struct {
	Action              processor.Action `json:"action"`
	SourceFormat        processor.Format `json:"sourceFormat"`
	StoredFormat        processor.Format `json:"storedFormat"`
	Preserved           bool             `json:"preserved"`
	CompressionEnabled  bool             `json:"compressionEnabled"`
	ConversionEnabled   bool             `json:"conversionEnabled"`
	CompressionRejected bool             `json:"compressionRejected,omitempty"`
}

func (s *Service) prepareImage(
	ctx context.Context,
	sourcePath string,
	sourceSize int64,
	options UploadOptions,
) (processor.Metadata, string, string, string, error) {
	sourceMetadata, err := processor.InspectFile(ctx, sourcePath, options.Limits)
	if err != nil {
		return processor.Metadata{}, "", "", "", err
	}
	metadata := sourceMetadata
	plan := processor.SelectPlan(sourceMetadata, options.Processing)
	selectedPath := sourcePath
	compressionRejected := false
	if plan.Action != processor.ActionPreserve {
		output, err := s.storage.CreateTemporary()
		if err != nil {
			return processor.Metadata{}, "", "", "", err
		}
		outputPath := output.Name()
		if err := output.Close(); err != nil {
			s.storage.RemoveTemporary(outputPath)
			return processor.Metadata{}, "", "", "", err
		}
		if err := s.processor.Transform(sourcePath, outputPath, sourceMetadata, options.Processing, plan); err != nil {
			s.storage.RemoveTemporary(outputPath)
			return processor.Metadata{}, "", "", "", err
		}
		outputInfo, err := os.Stat(outputPath)
		if err != nil {
			s.storage.RemoveTemporary(outputPath)
			return processor.Metadata{}, "", "", "", err
		}
		if plan.Action == processor.ActionCompress && outputInfo.Size() >= sourceSize {
			compressionRejected = true
			s.storage.RemoveTemporary(outputPath)
			plan = processor.Plan{Action: processor.ActionPreserve, OutputFormat: sourceMetadata.Format}
		} else {
			selectedPath = outputPath
			metadata, err = processor.InspectFile(ctx, selectedPath, options.Limits)
			if err != nil {
				s.storage.RemoveTemporary(outputPath)
				return processor.Metadata{}, "", "", "", err
			}
			if metadata.Format != plan.OutputFormat || metadata.Width != sourceMetadata.Width || metadata.Height != sourceMetadata.Height {
				s.storage.RemoveTemporary(outputPath)
				return processor.Metadata{}, "", "", "", processor.ErrInvalidImage
			}
		}
	}

	thumbnail, err := s.storage.CreateTemporary()
	if err != nil {
		if selectedPath != sourcePath {
			s.storage.RemoveTemporary(selectedPath)
		}
		return processor.Metadata{}, "", "", "", err
	}
	thumbnailPath := thumbnail.Name()
	if err := thumbnail.Close(); err != nil {
		s.storage.RemoveTemporary(thumbnailPath)
		if selectedPath != sourcePath {
			s.storage.RemoveTemporary(selectedPath)
		}
		return processor.Metadata{}, "", "", "", err
	}
	if err := s.processor.Thumbnail(selectedPath, thumbnailPath); err != nil {
		s.storage.RemoveTemporary(thumbnailPath)
		thumbnailPath = ""
		if !errors.Is(err, processor.ErrUnavailable) {
			if selectedPath != sourcePath {
				s.storage.RemoveTemporary(selectedPath)
			}
			return processor.Metadata{}, "", "", "", err
		}
	}
	summaryBytes, err := json.Marshal(processingSummary{
		Action: plan.Action, SourceFormat: sourceMetadata.Format, StoredFormat: metadata.Format,
		Preserved:           plan.Action == processor.ActionPreserve,
		CompressionEnabled:  options.Processing.CompressionEnabled,
		ConversionEnabled:   options.Processing.ConversionEnabled,
		CompressionRejected: compressionRejected,
	})
	if err != nil {
		if selectedPath != sourcePath {
			s.storage.RemoveTemporary(selectedPath)
		}
		if thumbnailPath != "" {
			s.storage.RemoveTemporary(thumbnailPath)
		}
		return processor.Metadata{}, "", "", "", err
	}
	return metadata, selectedPath, thumbnailPath, string(summaryBytes), nil
}

func hashFile(path string) (int64, [32]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, [32]byte{}, fmt.Errorf("open processed image for hashing: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return 0, [32]byte{}, fmt.Errorf("hash processed image: %w", err)
	}
	var digest [32]byte
	copy(digest[:], hasher.Sum(nil))
	return size, digest, nil
}

func mapProcessorError(err error) error {
	switch {
	case errors.Is(err, processor.ErrFileTooLarge):
		return ErrFileTooLarge
	case errors.Is(err, processor.ErrInvalidImage):
		return ErrInvalidImage
	case errors.Is(err, processor.ErrUnsupportedFormat):
		return ErrUnsupportedFormat
	case errors.Is(err, processor.ErrTooManyPixels):
		return ErrTooManyPixels
	case errors.Is(err, processor.ErrUnavailable):
		return ErrProcessingUnavailable
	default:
		return err
	}
}

func (s *Service) List(ctx context.Context, limit int) ([]Image, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repository.List(ctx, limit)
}

type listCursor struct {
	CreatedAt int64  `json:"createdAt"`
	ID        string `json:"id"`
}

func (s *Service) Search(ctx context.Context, filter ListFilter) (Page, error) {
	if err := validateListFilter(filter); err != nil {
		return Page{}, err
	}
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	repositoryFilter := repositoryListFilter{
		Limit: filter.Limit + 1, Query: strings.TrimSpace(filter.Query), Visibility: filter.Visibility,
		MIMEType: filter.MIMEType, UploadedVia: filter.UploadedVia, CreatedFrom: filter.CreatedFrom, CreatedTo: filter.CreatedTo,
		MinStoredBytes: filter.MinStoredBytes, MaxStoredBytes: filter.MaxStoredBytes,
		MinWidth: filter.MinWidth, MaxWidth: filter.MaxWidth, MinHeight: filter.MinHeight, MaxHeight: filter.MaxHeight,
	}
	if filter.Cursor != "" {
		cursor, err := decodeListCursor(filter.Cursor)
		if err != nil {
			return Page{}, ErrInvalidListFilter
		}
		repositoryFilter.BeforeUnix = cursor.CreatedAt
		repositoryFilter.BeforeID = cursor.ID
	}
	items, err := s.repository.ListFiltered(ctx, repositoryFilter)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(items) > filter.Limit {
		page.Items = items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor, err = encodeListCursor(listCursor{CreatedAt: last.CreatedAt.Unix(), ID: last.ID})
		if err != nil {
			return Page{}, err
		}
	}
	return page, nil
}

func (s *Service) Get(ctx context.Context, id string) (Image, error) {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return Image{}, ErrImageNotFound
	}
	return s.repository.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) (DeleteResult, error) {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return DeleteResult{}, ErrImageNotFound
	}
	releaseChange := s.barrier.BeginChange()
	deleted, err := s.repository.Delete(ctx, id)
	if err != nil {
		releaseChange()
		return DeleteResult{}, err
	}
	s.index.RemoveImage(id)
	releaseChange()

	result := DeleteResult{ImageID: id, ImageFileDeleted: true, ThumbnailDeleted: true}
	if err := s.storage.Remove(deleted.StorageKey); err != nil {
		result.ImageFileDeleted = false
		result.ImageCleanupError = err
	}
	if err := s.storage.RemoveThumbnail(id); err != nil {
		result.ThumbnailDeleted = false
		result.ThumbCleanupError = err
	}
	result.CleanupPending = !result.ImageFileDeleted || !result.ThumbnailDeleted
	return result, nil
}

func (s *Service) ChangeVisibility(ctx context.Context, id string, visibility Visibility) (bool, error) {
	if visibility != VisibilityPublic && visibility != VisibilityPrivate {
		return false, fmt.Errorf("invalid image visibility")
	}
	releaseChange := s.barrier.BeginChange()
	defer releaseChange()
	updated, err := s.repository.UpdateVisibility(ctx, id, visibility)
	if err != nil || !updated {
		return updated, err
	}
	s.index.UpdateVisibility(id, string(visibility))
	return true, nil
}

func sanitizeOriginalName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." {
		return "image"
	}
	runes := []rune(name)
	if len(runes) > 255 {
		name = string(runes[:255])
	}
	return name
}

func validateListFilter(filter ListFilter) error {
	if len([]rune(strings.TrimSpace(filter.Query))) > 200 ||
		(filter.Visibility != "" && filter.Visibility != VisibilityPublic && filter.Visibility != VisibilityPrivate) ||
		(filter.MIMEType != "" && filter.MIMEType != "image/jpeg" && filter.MIMEType != "image/png" && filter.MIMEType != "image/webp" && filter.MIMEType != "image/gif") ||
		(filter.UploadedVia != "" && filter.UploadedVia != "admin" && filter.UploadedVia != "api_token" && filter.UploadedVia != "import") ||
		filter.MinStoredBytes < 0 || filter.MaxStoredBytes < 0 || filter.MinWidth < 0 || filter.MaxWidth < 0 || filter.MinHeight < 0 || filter.MaxHeight < 0 ||
		(filter.MaxStoredBytes > 0 && filter.MinStoredBytes > filter.MaxStoredBytes) ||
		(filter.MaxWidth > 0 && filter.MinWidth > filter.MaxWidth) ||
		(filter.MaxHeight > 0 && filter.MinHeight > filter.MaxHeight) ||
		(filter.CreatedFrom != nil && filter.CreatedTo != nil && filter.CreatedFrom.After(*filter.CreatedTo)) {
		return ErrInvalidListFilter
	}
	return nil
}

func encodeListCursor(cursor listCursor) (string, error) {
	value, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode image list cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func decodeListCursor(value string) (listCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return listCursor{}, err
	}
	var cursor listCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return listCursor{}, err
	}
	id, err := uuid.Parse(cursor.ID)
	if err != nil || id.String() != cursor.ID || cursor.CreatedAt <= 0 {
		return listCursor{}, ErrInvalidListFilter
	}
	return cursor, nil
}
