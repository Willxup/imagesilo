package maintenance

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/indexstate"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

const maxReportedMissingIDs = 100

type IndexStats struct {
	Images   int
	Aliases  int
	Sessions int
	Tokens   int
}

type Overview struct {
	Persistent      PersistentStats
	Runtime         RuntimeSnapshot
	Indexes         IndexStats
	IndexConsistent bool
	LastInspection  *InspectionResult
	LastRebuild     *RebuildResult
}

type InspectionResult struct {
	CheckedAt            time.Time
	DatabaseImages       int
	ImageFiles           int
	ThumbnailFiles       int
	TemporaryFiles       int
	MissingImageCount    int
	MissingImageIDs      []string
	OrphanImageCount     int
	OrphanThumbnailCount int
}

type RebuildResult struct {
	CompletedAt       time.Time
	Images            int
	Aliases           int
	Sessions          int
	Tokens            int
	MissingImageCount int
	MissingImageIDs   []string
}

type Service struct {
	repository     *Repository
	storage        *storage.Filesystem
	rebuilder      *indexstate.Rebuilder
	delivery       *delivery.Index
	sessions       *auth.Service
	tokens         *apitoken.Service
	logger         *slog.Logger
	mu             sync.RWMutex
	lastInspection *InspectionResult
	lastRebuild    *RebuildResult
}

func NewService(
	repository *Repository,
	filesystem *storage.Filesystem,
	rebuilder *indexstate.Rebuilder,
	deliveryIndex *delivery.Index,
	sessions *auth.Service,
	tokens *apitoken.Service,
	logger *slog.Logger,
) *Service {
	return &Service{
		repository: repository, storage: filesystem, rebuilder: rebuilder,
		delivery: deliveryIndex, sessions: sessions, tokens: tokens, logger: logger,
	}
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	persistent, err := s.repository.Stats(ctx, time.Now())
	if err != nil {
		return Overview{}, err
	}
	indexes := IndexStats{
		Images: s.delivery.Len(), Aliases: s.delivery.AliasLen(),
		Sessions: s.sessions.SessionCount(), Tokens: s.tokens.TokenCount(),
	}
	s.mu.RLock()
	lastInspection := cloneInspection(s.lastInspection)
	lastRebuild := cloneRebuild(s.lastRebuild)
	s.mu.RUnlock()
	return Overview{
		Persistent: persistent, Runtime: CaptureRuntime(), Indexes: indexes,
		IndexConsistent: int64(indexes.Images) == persistent.ImageCount && int64(indexes.Aliases) == persistent.AliasCount &&
			int64(indexes.Sessions) == persistent.ActiveSessions && int64(indexes.Tokens) == persistent.ActiveTokens,
		LastInspection: lastInspection, LastRebuild: lastRebuild,
	}, nil
}

func (s *Service) Rebuild(ctx context.Context, now time.Time) (RebuildResult, error) {
	result, err := s.rebuilder.Rebuild(ctx, now)
	if err != nil {
		return RebuildResult{}, err
	}
	value := RebuildResult{
		CompletedAt: now.UTC(), Images: len(result.Delivery.LoadedIDs), Aliases: result.Delivery.LoadedAliasCount,
		Sessions: result.Sessions, Tokens: result.Tokens, MissingImageCount: len(result.Delivery.MissingIDs),
		MissingImageIDs: appendLimited(nil, result.Delivery.MissingIDs...),
	}
	s.mu.Lock()
	s.lastRebuild = &value
	s.mu.Unlock()
	s.logger.Info("in-memory indexes rebuilt",
		"images", value.Images, "aliases", value.Aliases, "sessions", value.Sessions, "api_tokens", value.Tokens,
		"missing_images", value.MissingImageCount,
	)
	return value, nil
}

func (s *Service) Inspect(ctx context.Context, now time.Time) (InspectionResult, error) {
	records, err := s.repository.ImageFiles(ctx)
	if err != nil {
		return InspectionResult{}, err
	}
	imageFiles, err := s.storage.ListImages()
	if err != nil {
		return InspectionResult{}, err
	}
	thumbnailFiles, err := s.storage.ListThumbnails()
	if err != nil {
		return InspectionResult{}, err
	}
	temporaryFiles, err := s.storage.ListTemporary()
	if err != nil {
		return InspectionResult{}, err
	}
	byStorageKey := make(map[string]string, len(records))
	imageIDs := make(map[string]struct{}, len(records))
	for _, record := range records {
		byStorageKey[record.StorageKey] = record.ID
		imageIDs[record.ID] = struct{}{}
	}
	present := make(map[string]struct{}, len(imageFiles))
	result := InspectionResult{
		CheckedAt: now.UTC(), DatabaseImages: len(records), ImageFiles: len(imageFiles),
		ThumbnailFiles: len(thumbnailFiles), TemporaryFiles: len(temporaryFiles), MissingImageIDs: []string{},
	}
	for _, entry := range imageFiles {
		present[entry.Key] = struct{}{}
		if _, exists := byStorageKey[entry.Key]; !exists || !entry.Regular {
			result.OrphanImageCount++
		}
	}
	for _, record := range records {
		if _, exists := present[record.StorageKey]; !exists {
			result.MissingImageCount++
			result.MissingImageIDs = appendLimited(result.MissingImageIDs, record.ID)
		}
	}
	for _, entry := range thumbnailFiles {
		if _, exists := imageIDs[entry.Key]; !exists || !entry.Regular {
			result.OrphanThumbnailCount++
		}
	}
	s.mu.Lock()
	s.lastInspection = &result
	s.mu.Unlock()
	s.logger.Info("manual consistency inspection completed",
		"database_images", result.DatabaseImages, "image_files", result.ImageFiles,
		"missing_images", result.MissingImageCount, "orphan_images", result.OrphanImageCount,
		"orphan_thumbnails", result.OrphanThumbnailCount, "temporary_files", result.TemporaryFiles,
	)
	return result, nil
}

func appendLimited(values []string, additions ...string) []string {
	remaining := maxReportedMissingIDs - len(values)
	if remaining <= 0 {
		return values
	}
	if len(additions) > remaining {
		additions = additions[:remaining]
	}
	return append(values, additions...)
}

func cloneInspection(value *InspectionResult) *InspectionResult {
	if value == nil {
		return nil
	}
	copy := *value
	copy.MissingImageIDs = append([]string(nil), value.MissingImageIDs...)
	return &copy
}

func cloneRebuild(value *RebuildResult) *RebuildResult {
	if value == nil {
		return nil
	}
	copy := *value
	copy.MissingImageIDs = append([]string(nil), value.MissingImageIDs...)
	return &copy
}
