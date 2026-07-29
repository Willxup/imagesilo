package image

import (
	"context"
	"crypto/sha256"
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
