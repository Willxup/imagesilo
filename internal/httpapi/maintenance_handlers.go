package httpapi

import (
	"net/http"
	"time"

	"github.com/Willxup/imagesilo/internal/maintenance"
)

type maintenanceHandler struct {
	service       *maintenance.Service
	authenticator *authenticator
}

type indexStatsResponse struct {
	Images   int `json:"images"`
	Aliases  int `json:"aliases"`
	Sessions int `json:"sessions"`
	Tokens   int `json:"tokens"`
}

type inspectionResponse struct {
	CheckedAt            time.Time `json:"checkedAt"`
	DatabaseImages       int       `json:"databaseImages"`
	ImageFiles           int       `json:"imageFiles"`
	ThumbnailFiles       int       `json:"thumbnailFiles"`
	TemporaryFiles       int       `json:"temporaryFiles"`
	MissingImageCount    int       `json:"missingImageCount"`
	MissingImageIDs      []string  `json:"missingImageIds"`
	OrphanImageCount     int       `json:"orphanImageCount"`
	OrphanThumbnailCount int       `json:"orphanThumbnailCount"`
}

type rebuildResponse struct {
	CompletedAt       time.Time `json:"completedAt"`
	Images            int       `json:"images"`
	Aliases           int       `json:"aliases"`
	Sessions          int       `json:"sessions"`
	Tokens            int       `json:"tokens"`
	MissingImageCount int       `json:"missingImageCount"`
	MissingImageIDs   []string  `json:"missingImageIds"`
}

type overviewResponse struct {
	ImageCount           int64               `json:"imageCount"`
	StoredBytes          int64               `json:"storedBytes"`
	MigrationStoredBytes int64               `json:"migrationStoredBytes"`
	AliasCount           int64               `json:"aliasCount"`
	HeapAllocBytes       uint64              `json:"heapAllocBytes"`
	HeapSysBytes         uint64              `json:"heapSysBytes"`
	RSSBytes             uint64              `json:"rssBytes"`
	Goroutines           int                 `json:"goroutines"`
	Indexes              indexStatsResponse  `json:"indexes"`
	IndexConsistent      bool                `json:"indexConsistent"`
	MissingImageCount    int                 `json:"missingImageCount"`
	MissingImageIDs      []string            `json:"missingImageIds"`
	LastInspection       *inspectionResponse `json:"lastInspection"`
	LastRebuild          *rebuildResponse    `json:"lastRebuild"`
	LastDaily            *dailyResponse      `json:"lastDaily"`
}

type dailyResponse struct {
	CompletedAt             time.Time          `json:"completedAt"`
	Inspection              inspectionResponse `json:"inspection"`
	RemovedTemporaryFiles   int                `json:"removedTemporaryFiles"`
	RemovedOrphanImages     int                `json:"removedOrphanImages"`
	RemovedOrphanThumbnails int                `json:"removedOrphanThumbnails"`
	CleanupFailures         int                `json:"cleanupFailures"`
	IndexConsistent         bool               `json:"indexConsistent"`
}

func newMaintenanceHandler(service *maintenance.Service, authenticator *authenticator) *maintenanceHandler {
	return &maintenanceHandler{service: service, authenticator: authenticator}
}

func (h *maintenanceHandler) overview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, false); !ok {
		return
	}
	value, err := h.service.Overview(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read system overview.")
		return
	}
	writeJSON(w, http.StatusOK, toOverviewResponse(value))
}

func (h *maintenanceHandler) rebuild(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	value, err := h.service.Rebuild(r.Context(), time.Now())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "rebuild_failed", "Unable to rebuild in-memory indexes.")
		return
	}
	writeJSON(w, http.StatusOK, toRebuildResponse(value))
}

func (h *maintenanceHandler) inspect(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	value, err := h.service.Inspect(r.Context(), time.Now())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "inspection_failed", "Unable to inspect data consistency.")
		return
	}
	writeJSON(w, http.StatusOK, toInspectionResponse(value))
}

func toOverviewResponse(value maintenance.Overview) overviewResponse {
	response := overviewResponse{
		ImageCount: value.Persistent.ImageCount, StoredBytes: value.Persistent.StoredBytes,
		MigrationStoredBytes: value.MigrationStoredBytes, AliasCount: value.Persistent.AliasCount,
		HeapAllocBytes: value.Runtime.HeapAllocBytes, HeapSysBytes: value.Runtime.HeapSysBytes,
		RSSBytes: value.Runtime.RSSBytes, Goroutines: value.Runtime.Goroutines,
		Indexes: indexStatsResponse{
			Images: value.Indexes.Images, Aliases: value.Indexes.Aliases,
			Sessions: value.Indexes.Sessions, Tokens: value.Indexes.Tokens,
		},
		IndexConsistent: value.IndexConsistent, MissingImageCount: value.MissingImageCount,
		MissingImageIDs: append([]string(nil), value.MissingImageIDs...),
	}
	if value.LastInspection != nil {
		inspection := toInspectionResponse(*value.LastInspection)
		response.LastInspection = &inspection
	}
	if value.LastRebuild != nil {
		rebuild := toRebuildResponse(*value.LastRebuild)
		response.LastRebuild = &rebuild
	}
	if value.LastDaily != nil {
		daily := dailyResponse{
			CompletedAt: value.LastDaily.CompletedAt, Inspection: toInspectionResponse(value.LastDaily.Inspection),
			RemovedTemporaryFiles:   value.LastDaily.RemovedTemporaryFiles,
			RemovedOrphanImages:     value.LastDaily.RemovedOrphanImages,
			RemovedOrphanThumbnails: value.LastDaily.RemovedOrphanThumbnails,
			CleanupFailures:         value.LastDaily.CleanupFailures, IndexConsistent: value.LastDaily.IndexConsistent,
		}
		response.LastDaily = &daily
	}
	return response
}

func toInspectionResponse(value maintenance.InspectionResult) inspectionResponse {
	return inspectionResponse{
		CheckedAt: value.CheckedAt, DatabaseImages: value.DatabaseImages, ImageFiles: value.ImageFiles,
		ThumbnailFiles: value.ThumbnailFiles, TemporaryFiles: value.TemporaryFiles,
		MissingImageCount: value.MissingImageCount, MissingImageIDs: value.MissingImageIDs,
		OrphanImageCount: value.OrphanImageCount, OrphanThumbnailCount: value.OrphanThumbnailCount,
	}
}

func toRebuildResponse(value maintenance.RebuildResult) rebuildResponse {
	return rebuildResponse{
		CompletedAt: value.CompletedAt, Images: value.Images, Aliases: value.Aliases,
		Sessions: value.Sessions, Tokens: value.Tokens,
		MissingImageCount: value.MissingImageCount, MissingImageIDs: value.MissingImageIDs,
	}
}
