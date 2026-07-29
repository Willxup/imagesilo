package image

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/google/uuid"
)

const (
	maxUploadBytes = 20 << 20
	maxTotalPixels = 40_000_000
)

type Service struct {
	repository *Repository
	storage    *storage.Filesystem
	index      *delivery.Index
}

func NewService(repository *Repository, filesystem *storage.Filesystem, index *delivery.Index) *Service {
	return &Service{repository: repository, storage: filesystem, index: index}
}

func (s *Service) UploadJPEG(ctx context.Context, reader io.Reader, originalName string, now time.Time) (Image, error) {
	temporary, err := s.storage.CreateTemporary()
	if err != nil {
		return Image{}, err
	}
	temporaryPath := temporary.Name()
	defer s.storage.RemoveTemporary(temporaryPath)

	hasher := sha256.New()
	limited := io.LimitReader(reader, maxUploadBytes+1)
	buffer := make([]byte, 64*1024)
	written, copyErr := io.CopyBuffer(io.MultiWriter(temporary, hasher), limited, buffer)
	closeErr := temporary.Close()
	if copyErr != nil {
		return Image{}, fmt.Errorf("stream upload: %w", copyErr)
	}
	if closeErr != nil {
		return Image{}, fmt.Errorf("close upload temporary file: %w", closeErr)
	}
	if written == 0 {
		return Image{}, ErrInvalidJPEG
	}
	if written > maxUploadBytes {
		return Image{}, ErrFileTooLarge
	}

	width, height, err := inspectJPEG(temporaryPath)
	if err != nil {
		return Image{}, err
	}
	if int64(width)*int64(height) > maxTotalPixels {
		return Image{}, ErrTooManyPixels
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Image{}, fmt.Errorf("generate image id: %w", err)
	}
	storageKey := id.String()
	if _, err := s.storage.CommitTemporary(temporaryPath, storageKey); err != nil {
		return Image{}, err
	}
	committed := true
	defer func() {
		if !committed {
			_ = s.storage.Remove(storageKey)
		}
	}()

	var digest [32]byte
	copy(digest[:], hasher.Sum(nil))
	value := Image{
		ID:                id.String(),
		OriginalName:      sanitizeOriginalName(originalName),
		StorageKey:        storageKey,
		Extension:         ".jpg",
		MIMEType:          "image/jpeg",
		Width:             width,
		Height:            height,
		SourceSize:        written,
		StoredSize:        written,
		SourceSHA256:      digest,
		StoredSHA256:      digest,
		ProcessingSummary: `{"preserved":true}`,
		Visibility:        VisibilityPublic,
		UploadedVia:       "admin",
		CreatedAt:         now.UTC(),
	}
	if err := s.repository.Create(ctx, value); err != nil {
		committed = false
		return Image{}, err
	}
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

func (s *Service) List(ctx context.Context, limit int) ([]Image, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repository.List(ctx, limit)
}

func inspectJPEG(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open uploaded image for inspection: %w", err)
	}
	reader := bufio.NewReader(file)
	header, err := reader.Peek(512)
	if err != nil && err != io.EOF {
		file.Close()
		return 0, 0, ErrInvalidJPEG
	}
	if http.DetectContentType(header) != "image/jpeg" {
		file.Close()
		return 0, 0, ErrInvalidJPEG
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return 0, 0, fmt.Errorf("rewind uploaded image: %w", err)
	}
	configuration, err := jpeg.DecodeConfig(file)
	file.Close()
	if err != nil || configuration.Width <= 0 || configuration.Height <= 0 {
		return 0, 0, ErrInvalidJPEG
	}
	if int64(configuration.Width)*int64(configuration.Height) > maxTotalPixels {
		return 0, 0, ErrTooManyPixels
	}

	file, err = os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("reopen uploaded image: %w", err)
	}
	decoded, err := jpeg.Decode(file)
	file.Close()
	if err != nil {
		return 0, 0, ErrInvalidJPEG
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != configuration.Width || bounds.Dy() != configuration.Height {
		return 0, 0, ErrInvalidJPEG
	}
	return configuration.Width, configuration.Height, nil
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
		return "image.jpg"
	}
	runes := []rune(name)
	if len(runes) > 255 {
		name = string(runes[:255])
	}
	return name
}
