package migrationimage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

var (
	ErrInvalidListFilter = errors.New("invalid migration image list filter")
	ErrInvalidImagePath  = errors.New("invalid migration image path")
	ErrImageNotFound     = errors.New("migration image not found")
	ErrMutationsDisabled = errors.New("migration image mutations are disabled")
)

const snapshotTTL = 30 * time.Minute

type Image struct {
	Path         string
	OriginalName string
	MIMEType     string
	Extension    string
	StoredSize   int64
	ModifiedAt   time.Time
	relativePath string
}

type ListFilter struct {
	Limit          int
	Cursor         string
	Query          string
	MIMEType       string
	MinStoredBytes int64
	MaxStoredBytes int64
}

type Page struct {
	Items            []Image
	NextCursor       string
	SkippedFiles     int
	MutationsEnabled bool
}

type DeleteResult struct {
	Path                    string
	RemovedDirectories      int
	DirectoryCleanupPending bool
	DirectoryCleanupError   error
}

type Service struct {
	storage          *storage.Filesystem
	mutationsEnabled bool
	mutationMu       sync.Mutex
	scanMu           sync.Mutex
	cacheMu          sync.RWMutex
	snapshot         *migrationSnapshot
	now              func() time.Time
}

type migrationSnapshot struct {
	items        []Image
	skippedFiles int
	scannedAt    time.Time
}

type listCursor struct {
	ModifiedAt int64  `json:"modifiedAt"`
	Path       string `json:"path"`
}

func NewService(filesystem *storage.Filesystem, mutationsEnabled bool) *Service {
	return &Service{storage: filesystem, mutationsEnabled: mutationsEnabled, now: time.Now}
}

func (s *Service) MutationsEnabled() bool {
	return s.mutationsEnabled
}

func (s *Service) Refresh(ctx context.Context) error {
	_, err := s.loadSnapshot(ctx, true)
	return err
}

func (s *Service) Search(ctx context.Context, filter ListFilter) (Page, error) {
	if err := validateListFilter(filter); err != nil {
		return Page{}, err
	}
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 24
	}
	var cursor listCursor
	if filter.Cursor != "" {
		var err error
		cursor, err = decodeCursor(filter.Cursor)
		if err != nil {
			return Page{}, ErrInvalidListFilter
		}
	}

	snapshot, err := s.loadSnapshot(ctx, false)
	if err != nil {
		return Page{}, err
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	items := make([]Image, 0, len(snapshot.items))
	for _, image := range snapshot.items {
		if err := ctx.Err(); err != nil {
			return Page{}, err
		}
		if query != "" && !strings.Contains(strings.ToLower(image.Path), query) &&
			!strings.Contains(strings.ToLower(image.relativePath), query) {
			continue
		}
		if filter.MIMEType != "" && image.MIMEType != filter.MIMEType {
			continue
		}
		if filter.MinStoredBytes > 0 && image.StoredSize < filter.MinStoredBytes {
			continue
		}
		if filter.MaxStoredBytes > 0 && image.StoredSize > filter.MaxStoredBytes {
			continue
		}
		items = append(items, image)
	}
	if filter.Cursor != "" {
		items = afterCursor(items, cursor)
	}

	page := Page{Items: items, SkippedFiles: snapshot.skippedFiles, MutationsEnabled: s.mutationsEnabled}
	if len(page.Items) > filter.Limit {
		page.Items = page.Items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor, err = encodeCursor(listCursor{ModifiedAt: last.ModifiedAt.UnixNano(), Path: last.Path})
		if err != nil {
			return Page{}, err
		}
	}
	return page, nil
}

func (s *Service) Delete(ctx context.Context, rawPath string) (DeleteResult, error) {
	if !s.mutationsEnabled {
		return DeleteResult{}, ErrMutationsDisabled
	}
	canonicalPath, relativePath, err := normalizeImagePath(rawPath)
	if err != nil {
		return DeleteResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return DeleteResult{}, err
	}

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	removed, err := s.storage.RemoveMigrationImage(relativePath)
	if errors.Is(err, fs.ErrNotExist) {
		s.removeCachedPath(canonicalPath)
		return DeleteResult{}, ErrImageNotFound
	}
	if errors.Is(err, storage.ErrUnsafeMigrationPath) {
		return DeleteResult{}, ErrInvalidImagePath
	}
	if err != nil {
		return DeleteResult{}, err
	}
	s.removeCachedPath(canonicalPath)
	return DeleteResult{
		Path: canonicalPath, RemovedDirectories: removed.RemovedDirectories,
		DirectoryCleanupPending: removed.DirectoryCleanupError != nil,
		DirectoryCleanupError:   removed.DirectoryCleanupError,
	}, nil
}

func (s *Service) loadSnapshot(ctx context.Context, force bool) (*migrationSnapshot, error) {
	if !force {
		if snapshot, ok := s.freshSnapshot(s.now()); ok {
			return snapshot, nil
		}
	}

	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if !force {
		if snapshot, ok := s.freshSnapshot(s.now()); ok {
			return snapshot, nil
		}
	}

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	scanned, err := s.scanSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	s.cacheMu.Lock()
	s.snapshot = scanned
	s.cacheMu.Unlock()
	return scanned, nil
}

func (s *Service) freshSnapshot(now time.Time) (*migrationSnapshot, bool) {
	s.cacheMu.RLock()
	snapshot := s.snapshot
	s.cacheMu.RUnlock()
	return snapshot, snapshot != nil && now.Before(snapshot.scannedAt.Add(snapshotTTL))
}

func (s *Service) scanSnapshot(ctx context.Context) (*migrationSnapshot, error) {
	scanned, err := s.storage.ListMigrationImages(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Image, 0, len(scanned.Files))
	skipped := scanned.SkippedFiles
	for _, file := range scanned.Files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		canonicalPath, err := delivery.NormalizeAliasPath("/" + file.RelativePath)
		if err != nil {
			skipped++
			continue
		}
		items = append(items, Image{
			Path: canonicalPath, OriginalName: path.Base(file.RelativePath), MIMEType: file.MIMEType,
			Extension: file.Extension, StoredSize: file.Size, ModifiedAt: file.ModifiedAt, relativePath: file.RelativePath,
		})
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].ModifiedAt.Equal(items[right].ModifiedAt) {
			return items[left].Path < items[right].Path
		}
		return items[left].ModifiedAt.After(items[right].ModifiedAt)
	})
	return &migrationSnapshot{items: items, skippedFiles: skipped, scannedAt: s.now().UTC()}, nil
}

func (s *Service) removeCachedPath(canonicalPath string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.snapshot == nil {
		return
	}
	items := make([]Image, 0, len(s.snapshot.items))
	for _, item := range s.snapshot.items {
		if item.Path != canonicalPath {
			items = append(items, item)
		}
	}
	s.snapshot = &migrationSnapshot{items: items, skippedFiles: s.snapshot.skippedFiles, scannedAt: s.snapshot.scannedAt}
}

func validateListFilter(filter ListFilter) error {
	if len(strings.TrimSpace(filter.Query)) > 256 || filter.MinStoredBytes < 0 || filter.MaxStoredBytes < 0 ||
		(filter.MinStoredBytes > 0 && filter.MaxStoredBytes > 0 && filter.MinStoredBytes > filter.MaxStoredBytes) {
		return ErrInvalidListFilter
	}
	if filter.MIMEType != "" {
		switch filter.MIMEType {
		case "image/jpeg", "image/png", "image/webp", "image/gif":
		default:
			return ErrInvalidListFilter
		}
	}
	return nil
}

func normalizeImagePath(rawPath string) (string, string, error) {
	canonicalPath, err := delivery.NormalizeAliasPath(strings.TrimSpace(rawPath))
	if err != nil {
		return "", "", ErrInvalidImagePath
	}
	decoded, err := url.PathUnescape(canonicalPath)
	if err != nil {
		return "", "", ErrInvalidImagePath
	}
	relativePath := strings.TrimPrefix(decoded, "/")
	if relativePath == "" || !fs.ValidPath(relativePath) {
		return "", "", ErrInvalidImagePath
	}
	return canonicalPath, relativePath, nil
}

func afterCursor(items []Image, cursor listCursor) []Image {
	for index, item := range items {
		modifiedAt := item.ModifiedAt.UnixNano()
		if modifiedAt < cursor.ModifiedAt || (modifiedAt == cursor.ModifiedAt && item.Path > cursor.Path) {
			return items[index:]
		}
	}
	return nil
}

func encodeCursor(value listCursor) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode migration image cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeCursor(raw string) (listCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return listCursor{}, err
	}
	var value listCursor
	if err := json.Unmarshal(decoded, &value); err != nil {
		return listCursor{}, ErrInvalidListFilter
	}
	normalized, _, err := normalizeImagePath(value.Path)
	if err != nil || normalized != value.Path {
		return listCursor{}, ErrInvalidListFilter
	}
	return value, nil
}
