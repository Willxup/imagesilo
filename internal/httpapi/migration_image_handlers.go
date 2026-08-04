package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Willxup/imagesilo/internal/migrationimage"
)

type migrationImageHandler struct {
	service       *migrationimage.Service
	authenticator *authenticator
	logger        *slog.Logger
}

type migrationImageResponse struct {
	Path         string    `json:"path"`
	OriginalName string    `json:"originalName"`
	MIMEType     string    `json:"mimeType"`
	Extension    string    `json:"extension"`
	StoredSize   int64     `json:"storedSize"`
	StandardURL  string    `json:"standardUrl"`
	ModifiedAt   time.Time `json:"modifiedAt"`
}

type migrationImageListResponse struct {
	Items            []migrationImageResponse `json:"items"`
	NextCursor       string                   `json:"nextCursor,omitempty"`
	SkippedFiles     int                      `json:"skippedFiles"`
	MutationsEnabled bool                     `json:"mutationsEnabled"`
}

type batchMigrationImageRequest struct {
	Paths []string `json:"paths"`
}

type migrationImageOperationItem struct {
	Path                    string `json:"path"`
	Status                  string `json:"status"`
	RemovedDirectories      int    `json:"removedDirectories,omitempty"`
	DirectoryCleanupPending bool   `json:"directoryCleanupPending,omitempty"`
	ErrorCode               string `json:"errorCode,omitempty"`
}

type migrationImageBatchResult struct {
	Items []migrationImageOperationItem `json:"items"`
}

func newMigrationImageHandler(service *migrationimage.Service, authenticator *authenticator, logger *slog.Logger) *migrationImageHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &migrationImageHandler{service: service, authenticator: authenticator, logger: logger}
}

func (h *migrationImageHandler) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, false); !ok {
		return
	}
	filter, err := parseMigrationImageListFilter(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_migration_image_filter", "Migration image list filter is invalid.")
		return
	}
	page, err := h.service.Search(r.Context(), filter)
	if err != nil {
		if errors.Is(err, migrationimage.ErrInvalidListFilter) {
			writeError(w, r, http.StatusBadRequest, "invalid_migration_image_filter", "Migration image list filter is invalid.")
			return
		}
		h.logger.Error("migration image scan failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "migration_scan_failed", "Unable to scan migration images.")
		return
	}
	response := migrationImageListResponse{
		Items: make([]migrationImageResponse, 0, len(page.Items)), NextCursor: page.NextCursor,
		SkippedFiles: page.SkippedFiles, MutationsEnabled: page.MutationsEnabled,
	}
	for _, value := range page.Items {
		response.Items = append(response.Items, migrationImageResponse{
			Path: value.Path, OriginalName: value.OriginalName, MIMEType: value.MIMEType,
			Extension: value.Extension, StoredSize: value.StoredSize, StandardURL: value.Path, ModifiedAt: value.ModifiedAt,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *migrationImageHandler) refresh(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	if err := h.service.Refresh(r.Context()); err != nil {
		h.logger.Error("migration image refresh failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "migration_refresh_failed", "Unable to refresh migration images.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *migrationImageHandler) batchDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	if !h.service.MutationsEnabled() {
		writeError(w, r, http.StatusForbidden, "migration_mutations_disabled", "Migration image deletion is disabled.")
		return
	}
	var request batchMigrationImageRequest
	if err := decodeMigrationImageBatch(w, r, &request); err != nil || len(request.Paths) == 0 || len(request.Paths) > 100 {
		writeError(w, r, http.StatusBadRequest, "invalid_migration_batch", "Batch delete requires 1 to 100 migration image paths.")
		return
	}
	seen := make(map[string]struct{}, len(request.Paths))
	for _, path := range request.Paths {
		if _, exists := seen[path]; exists {
			writeError(w, r, http.StatusBadRequest, "invalid_migration_batch", "Migration image paths must be unique.")
			return
		}
		seen[path] = struct{}{}
	}

	response := migrationImageBatchResult{Items: make([]migrationImageOperationItem, 0, len(request.Paths))}
	for _, path := range request.Paths {
		result, err := h.service.Delete(r.Context(), path)
		item := migrationImageOperationItem{Path: path, Status: "deleted"}
		switch {
		case errors.Is(err, migrationimage.ErrInvalidImagePath):
			item.Status = "error"
			item.ErrorCode = "invalid_migration_path"
		case errors.Is(err, migrationimage.ErrImageNotFound):
			item.Status = "not_found"
			item.ErrorCode = "migration_image_not_found"
		case err != nil:
			item.Status = "error"
			item.ErrorCode = "migration_delete_failed"
			h.logger.Warn("migration image delete failed", "path", path, "error", err)
		default:
			item.Path = result.Path
			item.RemovedDirectories = result.RemovedDirectories
			item.DirectoryCleanupPending = result.DirectoryCleanupPending
			if result.DirectoryCleanupPending {
				item.Status = "cleanup_pending"
				item.ErrorCode = "directory_cleanup_failed"
				h.logger.Warn("migration image empty-directory cleanup failed", "path", result.Path, "error", result.DirectoryCleanupError)
			}
		}
		response.Items = append(response.Items, item)
	}
	writeJSON(w, http.StatusOK, response)
}

func decodeMigrationImageBatch(w http.ResponseWriter, r *http.Request, destination *batchMigrationImageRequest) error {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

func parseMigrationImageListFilter(r *http.Request) (migrationimage.ListFilter, error) {
	query := r.URL.Query()
	limit := 24
	var err error
	if raw := query.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit <= 0 || limit > 100 {
			return migrationimage.ListFilter{}, migrationimage.ErrInvalidListFilter
		}
	}
	filter := migrationimage.ListFilter{Limit: limit, Cursor: query.Get("cursor"), Query: query.Get("q")}
	if raw := query.Get("format"); raw != "" {
		formats := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp", "gif": "image/gif"}
		mimeType, ok := formats[raw]
		if !ok {
			return migrationimage.ListFilter{}, migrationimage.ErrInvalidListFilter
		}
		filter.MIMEType = mimeType
	}
	if filter.MinStoredBytes, err = parseOptionalInt64(query.Get("minBytes")); err != nil {
		return migrationimage.ListFilter{}, err
	}
	if filter.MaxStoredBytes, err = parseOptionalInt64(query.Get("maxBytes")); err != nil {
		return migrationimage.ListFilter{}, err
	}
	return filter, nil
}
