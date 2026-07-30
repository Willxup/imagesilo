package importer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	imagealias "github.com/Willxup/imagesilo/internal/alias"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/google/uuid"
)

const (
	defaultMaxBytes  = 20 << 20
	defaultMaxPixels = 16_000_000
)

type Service struct {
	repository *Repository
	storage    *storage.Filesystem
	index      *delivery.Index
	processor  processor.Engine
	gate       *processor.Gate
	barrier    *indexbarrier.Barrier
}

func NewService(repository *Repository, filesystem *storage.Filesystem, index *delivery.Index, engine processor.Engine, gate *processor.Gate, barrier *indexbarrier.Barrier) *Service {
	return &Service{repository: repository, storage: filesystem, index: index, processor: engine, gate: gate, barrier: barrier}
}

func (s *Service) Import(ctx context.Context, reader io.Reader, originalName, aliasPath string, options Options, now time.Time) (Result, error) {
	if options.Visibility != images.VisibilityPublic && options.Visibility != images.VisibilityPrivate {
		return Result{}, fmt.Errorf("invalid image visibility")
	}
	normalizedAlias, err := delivery.NormalizeAliasPath(aliasPath)
	if err != nil {
		return Result{}, err
	}
	aliasExists, err := s.repository.AliasExists(ctx, normalizedAlias)
	if err != nil {
		return Result{}, err
	}
	if aliasExists {
		return Result{}, imagealias.ErrAliasConflict
	}
	if options.Limits.MaxBytes <= 0 {
		options.Limits.MaxBytes = defaultMaxBytes
	}
	if options.Limits.MaxTotalPixels <= 0 {
		options.Limits.MaxTotalPixels = defaultMaxPixels
	}

	temporary, err := s.storage.CreateTemporary()
	if err != nil {
		return Result{}, err
	}
	sourcePath := temporary.Name()
	defer s.storage.RemoveTemporary(sourcePath)
	hasher := sha256.New()
	written, copyErr := io.CopyBuffer(io.MultiWriter(temporary, hasher), io.LimitReader(reader, options.Limits.MaxBytes+1), make([]byte, 64*1024))
	closeErr := temporary.Close()
	if copyErr != nil {
		return Result{}, fmt.Errorf("stream imported image: %w", copyErr)
	}
	if closeErr != nil {
		return Result{}, fmt.Errorf("close imported image temporary file: %w", closeErr)
	}
	if written == 0 {
		return Result{}, images.ErrInvalidImage
	}
	if written > options.Limits.MaxBytes {
		return Result{}, images.ErrFileTooLarge
	}
	var digest [32]byte
	copy(digest[:], hasher.Sum(nil))

	release, ok := s.gate.TryAcquire()
	if !ok {
		return Result{}, images.ErrProcessingBusy
	}
	metadata, thumbnailPath, err := func() (processor.Metadata, string, error) {
		defer release()
		defer processor.TrimMemory()
		metadata, err := processor.InspectFile(ctx, sourcePath, options.Limits)
		if err != nil {
			return processor.Metadata{}, "", err
		}
		thumbnail, err := s.storage.CreateTemporary()
		if err != nil {
			return processor.Metadata{}, "", err
		}
		path := thumbnail.Name()
		if err := thumbnail.Close(); err != nil {
			s.storage.RemoveTemporary(path)
			return processor.Metadata{}, "", err
		}
		if err := s.processor.Thumbnail(sourcePath, path); err != nil {
			s.storage.RemoveTemporary(path)
			if errors.Is(err, processor.ErrUnavailable) {
				return metadata, "", nil
			}
			return processor.Metadata{}, "", err
		}
		return metadata, path, nil
	}()
	if err != nil {
		return Result{}, mapProcessorError(err)
	}
	if thumbnailPath != "" {
		defer s.storage.RemoveTemporary(thumbnailPath)
	}

	imageID, err := uuid.NewV7()
	if err != nil {
		return Result{}, fmt.Errorf("generate imported image id: %w", err)
	}
	aliasID, err := uuid.NewV7()
	if err != nil {
		return Result{}, fmt.Errorf("generate imported alias id: %w", err)
	}
	summary, err := json.Marshal(importSummary{
		Action: processor.ActionPreserve, SourceFormat: metadata.Format, StoredFormat: metadata.Format,
		Preserved: true, CompressionEnabled: false, ConversionEnabled: false,
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode import summary: %w", err)
	}
	value := images.Image{
		ID: imageID.String(), OriginalName: images.SanitizeOriginalName(originalName), StorageKey: imageID.String(),
		Extension: metadata.Extension, MIMEType: metadata.MIMEType, Width: metadata.Width, Height: metadata.Height,
		SourceSize: written, StoredSize: written, SourceSHA256: digest, StoredSHA256: digest,
		ProcessingSummary: string(summary), Visibility: options.Visibility, UploadedVia: "import",
		UploadedByAPITokenID: options.UploadedByAPITokenID, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	alias := importedAlias(aliasID.String(), normalizedAlias, value.ID, now)
	if _, err := s.storage.CommitTemporary(sourcePath, value.StorageKey); err != nil {
		return Result{}, err
	}
	databaseCommitted := false
	thumbnailCommitted := false
	defer func() {
		if !databaseCommitted {
			_ = s.storage.Remove(value.StorageKey)
			if thumbnailCommitted {
				_ = s.storage.RemoveThumbnail(value.ID)
			}
		}
	}()
	if thumbnailPath != "" {
		if err := s.storage.CommitThumbnailTemporary(thumbnailPath, value.ID); err != nil {
			return Result{}, err
		}
		thumbnailCommitted = true
	}
	releaseChange := s.barrier.BeginChange()
	defer releaseChange()
	if err := s.repository.Create(ctx, value, alias); err != nil {
		return Result{}, err
	}
	databaseCommitted = true
	s.index.Add(value.ID, delivery.Target{
		StorageKey: value.StorageKey, MIMEType: value.MIMEType, ETag: fmt.Sprintf("\"%x\"", value.StoredSHA256),
		Size: value.StoredSize, LastModified: value.UpdatedAt, Visibility: string(value.Visibility), OriginalName: value.OriginalName,
	})
	s.index.AddAlias(alias.Path, value.ID)
	return Result{Image: value, Alias: alias}, nil
}

func mapProcessorError(err error) error {
	switch {
	case errors.Is(err, processor.ErrFileTooLarge):
		return images.ErrFileTooLarge
	case errors.Is(err, processor.ErrInvalidImage):
		return images.ErrInvalidImage
	case errors.Is(err, processor.ErrUnsupportedFormat):
		return images.ErrUnsupportedFormat
	case errors.Is(err, processor.ErrTooManyPixels):
		return images.ErrTooManyPixels
	default:
		return err
	}
}
