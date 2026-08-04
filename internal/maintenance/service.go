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
	Persistent           PersistentStats
	MigrationStoredBytes int64
	Runtime              RuntimeSnapshot
	Indexes              IndexStats
	IndexConsistent      bool
	MissingImageCount    int
	MissingImageIDs      []string
	LastInspection       *InspectionResult
	LastRebuild          *RebuildResult
	LastDaily            *DailyResult
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

type DailyResult struct {
	CompletedAt             time.Time
	Inspection              InspectionResult
	RemovedTemporaryFiles   int
	RemovedOrphanImages     int
	RemovedOrphanThumbnails int
	CleanupFailures         int
	Persistent              PersistentStats
	Runtime                 RuntimeSnapshot
	Indexes                 IndexStats
	IndexConsistent         bool
}

type Service struct {
	repository        *Repository
	storage           *storage.Filesystem
	migrationStorage  migrationStorage
	rebuilder         *indexstate.Rebuilder
	delivery          *delivery.Index
	sessions          *auth.Service
	tokens            *apitoken.Service
	logger            *slog.Logger
	mu                sync.RWMutex
	missingImageCount int
	missingImageIDs   []string
	lastInspection    *InspectionResult
	lastRebuild       *RebuildResult
	lastDaily         *DailyResult
}

type migrationStorage interface {
	StoredBytes(context.Context) (int64, error)
}

func NewService(
	repository *Repository,
	filesystem *storage.Filesystem,
	rebuilder *indexstate.Rebuilder,
	deliveryIndex *delivery.Index,
	sessions *auth.Service,
	tokens *apitoken.Service,
	migrationStorage migrationStorage,
	logger *slog.Logger,
) *Service {
	return &Service{
		repository: repository, storage: filesystem, rebuilder: rebuilder,
		delivery: deliveryIndex, sessions: sessions, tokens: tokens, migrationStorage: migrationStorage, logger: logger,
	}
}

func (s *Service) RecordStartupMissing(ids []string) {
	s.mu.Lock()
	s.missingImageCount = len(ids)
	s.missingImageIDs = appendLimited(nil, ids...)
	s.mu.Unlock()
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	now := time.Now()
	persistent, indexes, consistent, err := s.status(ctx, now)
	if err != nil {
		return Overview{}, err
	}
	migrationBytes, err := s.migrationStorage.StoredBytes(ctx)
	if err != nil {
		return Overview{}, err
	}
	s.mu.RLock()
	missingCount := s.missingImageCount
	missingIDs := append([]string(nil), s.missingImageIDs...)
	lastInspection := cloneInspection(s.lastInspection)
	lastRebuild := cloneRebuild(s.lastRebuild)
	lastDaily := cloneDaily(s.lastDaily)
	s.mu.RUnlock()
	return Overview{
		Persistent: persistent, MigrationStoredBytes: migrationBytes, Runtime: CaptureRuntime(), Indexes: indexes, IndexConsistent: consistent,
		MissingImageCount: missingCount, MissingImageIDs: missingIDs,
		LastInspection: lastInspection, LastRebuild: lastRebuild, LastDaily: lastDaily,
	}, nil
}

func (s *Service) Rebuild(ctx context.Context, now time.Time) (RebuildResult, error) {
	result, err := s.rebuilder.Rebuild(ctx, now)
	if err != nil {
		return RebuildResult{}, err
	}
	value := RebuildResult{
		CompletedAt: now.UTC(), Images: len(result.Delivery.LoadedIDs), Aliases: result.Delivery.LoadedAliasCount,
		Sessions: result.Sessions, Tokens: result.Tokens,
	}
	unavailableIDs := append(append([]string(nil), result.Delivery.MissingIDs...), result.Delivery.InvalidSizeIDs...)
	value.MissingImageCount = len(unavailableIDs)
	value.MissingImageIDs = appendLimited(nil, unavailableIDs...)
	s.mu.Lock()
	s.lastRebuild = &value
	s.missingImageCount = value.MissingImageCount
	s.missingImageIDs = append([]string(nil), value.MissingImageIDs...)
	s.mu.Unlock()
	s.logger.Info("in-memory indexes rebuilt",
		"images", value.Images, "aliases", value.Aliases, "sessions", value.Sessions, "api_tokens", value.Tokens,
		"missing_images", value.MissingImageCount,
	)
	return value, nil
}

func (s *Service) Inspect(ctx context.Context, now time.Time) (InspectionResult, error) {
	result, _, err := s.scan(ctx, now)
	if err != nil {
		return InspectionResult{}, err
	}
	s.recordInspection(result)
	s.logger.Info("manual consistency inspection completed",
		"database_images", result.DatabaseImages, "image_files", result.ImageFiles,
		"missing_images", result.MissingImageCount, "orphan_images", result.OrphanImageCount,
		"orphan_thumbnails", result.OrphanThumbnailCount, "temporary_files", result.TemporaryFiles,
	)
	return result, nil
}

func (s *Service) CleanupStaleTemporary(now time.Time, safetyAge time.Duration) (int, int, error) {
	if safetyAge <= 0 {
		safetyAge = 24 * time.Hour
	}
	entries, err := s.storage.ListTemporary()
	if err != nil {
		return 0, 0, err
	}
	cutoff := now.Add(-safetyAge)
	removed := 0
	failures := 0
	for _, entry := range entries {
		if !entry.Regular || entry.ModifiedAt.After(cutoff) {
			continue
		}
		if err := s.storage.RemoveTemporaryKey(entry.Key); err != nil {
			failures++
			s.logger.Warn("startup temporary cleanup failed", "key", entry.Key, "error", err)
			continue
		}
		removed++
	}
	if removed > 0 || failures > 0 {
		s.logger.Info("startup temporary cleanup completed", "removed_temporary_files", removed, "cleanup_failures", failures)
	}
	return removed, failures, nil
}

func (s *Service) Daily(ctx context.Context, now time.Time, safetyAge time.Duration) (DailyResult, error) {
	if safetyAge <= 0 {
		safetyAge = 24 * time.Hour
	}
	inspection, state, err := s.scan(ctx, now)
	if err != nil {
		return DailyResult{}, err
	}
	result := DailyResult{CompletedAt: now.UTC(), Inspection: inspection}
	cutoff := now.Add(-safetyAge)
	for _, entry := range state.imageFiles {
		_, referenced := state.storageKeys[entry.Key]
		if referenced || !entry.Regular || entry.ModifiedAt.After(cutoff) {
			continue
		}
		if err := s.storage.Remove(entry.Key); err != nil {
			result.CleanupFailures++
			s.logger.Warn("daily maintenance cleanup failed", "kind", "orphan_image", "key", entry.Key, "error", err)
			continue
		}
		result.RemovedOrphanImages++
		result.Inspection.ImageFiles--
		result.Inspection.OrphanImageCount--
	}
	for _, entry := range state.thumbnailFiles {
		_, referenced := state.imageIDs[entry.Key]
		if referenced || !entry.Regular || entry.ModifiedAt.After(cutoff) {
			continue
		}
		if err := s.storage.RemoveThumbnail(entry.Key); err != nil {
			result.CleanupFailures++
			s.logger.Warn("daily maintenance cleanup failed", "kind", "orphan_thumbnail", "key", entry.Key, "error", err)
			continue
		}
		result.RemovedOrphanThumbnails++
		result.Inspection.ThumbnailFiles--
		result.Inspection.OrphanThumbnailCount--
	}
	for _, entry := range state.temporaryFiles {
		if !entry.Regular || entry.ModifiedAt.After(cutoff) {
			continue
		}
		if err := s.storage.RemoveTemporaryKey(entry.Key); err != nil {
			result.CleanupFailures++
			s.logger.Warn("daily maintenance cleanup failed", "kind", "temporary_file", "key", entry.Key, "error", err)
			continue
		}
		result.RemovedTemporaryFiles++
		result.Inspection.TemporaryFiles--
	}
	result.Persistent, result.Indexes, result.IndexConsistent, err = s.status(ctx, now)
	if err != nil {
		return DailyResult{}, err
	}
	result.Runtime = CaptureRuntime()
	s.mu.Lock()
	s.lastInspection = cloneInspection(&result.Inspection)
	s.lastDaily = cloneDaily(&result)
	s.missingImageCount = result.Inspection.MissingImageCount
	s.missingImageIDs = append([]string(nil), result.Inspection.MissingImageIDs...)
	s.mu.Unlock()
	s.logger.Info("daily maintenance completed",
		"removed_temporary_files", result.RemovedTemporaryFiles,
		"removed_orphan_images", result.RemovedOrphanImages,
		"removed_orphan_thumbnails", result.RemovedOrphanThumbnails,
		"cleanup_failures", result.CleanupFailures,
		"missing_images", result.Inspection.MissingImageCount,
		"image_count", result.Persistent.ImageCount, "alias_count", result.Persistent.AliasCount,
		"index_images", result.Indexes.Images, "index_aliases", result.Indexes.Aliases,
		"index_sessions", result.Indexes.Sessions, "index_tokens", result.Indexes.Tokens,
		"index_consistent", result.IndexConsistent,
		"go_heap_alloc_bytes", result.Runtime.HeapAllocBytes, "go_heap_sys_bytes", result.Runtime.HeapSysBytes,
		"rss_bytes", result.Runtime.RSSBytes, "goroutines", result.Runtime.Goroutines,
	)
	return result, nil
}

type scanState struct {
	storageKeys    map[string]string
	expectedSizes  map[string]int64
	imageIDs       map[string]struct{}
	imageFiles     []storage.FileEntry
	thumbnailFiles []storage.FileEntry
	temporaryFiles []storage.FileEntry
}

func (s *Service) scan(ctx context.Context, now time.Time) (InspectionResult, scanState, error) {
	records, err := s.repository.ImageFiles(ctx)
	if err != nil {
		return InspectionResult{}, scanState{}, err
	}
	imageFiles, err := s.storage.ListImages()
	if err != nil {
		return InspectionResult{}, scanState{}, err
	}
	thumbnailFiles, err := s.storage.ListThumbnails()
	if err != nil {
		return InspectionResult{}, scanState{}, err
	}
	temporaryFiles, err := s.storage.ListTemporary()
	if err != nil {
		return InspectionResult{}, scanState{}, err
	}
	state := scanState{
		storageKeys: make(map[string]string, len(records)), imageIDs: make(map[string]struct{}, len(records)),
		expectedSizes: make(map[string]int64, len(records)),
		imageFiles:    imageFiles, thumbnailFiles: thumbnailFiles, temporaryFiles: temporaryFiles,
	}
	for _, record := range records {
		state.storageKeys[record.StorageKey] = record.ID
		state.expectedSizes[record.StorageKey] = record.StoredSize
		state.imageIDs[record.ID] = struct{}{}
	}
	present := make(map[string]struct{}, len(imageFiles))
	result := InspectionResult{
		CheckedAt: now.UTC(), DatabaseImages: len(records), ImageFiles: len(imageFiles),
		ThumbnailFiles: len(thumbnailFiles), TemporaryFiles: len(temporaryFiles), MissingImageIDs: []string{},
	}
	for _, entry := range imageFiles {
		_, referenced := state.storageKeys[entry.Key]
		if entry.Regular && referenced && entry.Size == state.expectedSizes[entry.Key] {
			present[entry.Key] = struct{}{}
		}
		if !referenced || !entry.Regular {
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
		if _, exists := state.imageIDs[entry.Key]; !exists || !entry.Regular {
			result.OrphanThumbnailCount++
		}
	}
	return result, state, nil
}

func (s *Service) status(ctx context.Context, now time.Time) (PersistentStats, IndexStats, bool, error) {
	persistent, err := s.repository.Stats(ctx, now)
	if err != nil {
		return PersistentStats{}, IndexStats{}, false, err
	}
	indexes := IndexStats{
		Images: s.delivery.Len(), Aliases: s.delivery.AliasLen(),
		Sessions: s.sessions.SessionCount(), Tokens: s.tokens.TokenCount(),
	}
	consistent := int64(indexes.Images) == persistent.ImageCount && int64(indexes.Aliases) == persistent.AliasCount &&
		int64(indexes.Sessions) == persistent.ActiveSessions && int64(indexes.Tokens) == persistent.ActiveTokens
	return persistent, indexes, consistent, nil
}

func (s *Service) recordInspection(result InspectionResult) {
	s.mu.Lock()
	s.lastInspection = cloneInspection(&result)
	s.missingImageCount = result.MissingImageCount
	s.missingImageIDs = append([]string(nil), result.MissingImageIDs...)
	s.mu.Unlock()
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

func cloneDaily(value *DailyResult) *DailyResult {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Inspection.MissingImageIDs = append([]string(nil), value.Inspection.MissingImageIDs...)
	return &copy
}
