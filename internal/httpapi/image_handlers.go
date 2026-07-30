package httpapi

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	imagealias "github.com/Willxup/imagesilo/internal/alias"
	"github.com/Willxup/imagesilo/internal/apitoken"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/maintenance"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/Willxup/imagesilo/internal/settings"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const multipartOverheadBytes = 2 << 20

type imageHandler struct {
	service       *images.Service
	aliases       *imagealias.Service
	settings      *settings.Service
	storage       *storage.Filesystem
	authenticator *authenticator
	logger        *slog.Logger
}

type imageResponse struct {
	ID                string            `json:"id"`
	OriginalName      string            `json:"originalName"`
	MIMEType          string            `json:"mimeType"`
	Extension         string            `json:"extension"`
	Width             int               `json:"width"`
	Height            int               `json:"height"`
	SourceSize        int64             `json:"sourceSize"`
	StoredSize        int64             `json:"storedSize"`
	SourceSHA256      string            `json:"sourceSha256"`
	StoredSHA256      string            `json:"storedSha256"`
	Visibility        images.Visibility `json:"visibility"`
	UploadedVia       string            `json:"uploadedVia"`
	StandardURL       string            `json:"standardUrl"`
	ThumbnailURL      string            `json:"thumbnailUrl"`
	ProcessingSummary json.RawMessage   `json:"processingSummary"`
	CreatedAt         time.Time         `json:"createdAt"`
}

type imageListResponse struct {
	Items      []imageResponse `json:"items"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

type imageDetailResponse struct {
	imageResponse
	Aliases []aliasResponse `json:"aliases"`
}

type visibilityRequest struct {
	Visibility images.Visibility `json:"visibility"`
}

type batchImageRequest struct {
	ImageIDs []string `json:"imageIds"`
}

type batchVisibilityRequest struct {
	ImageIDs   []string          `json:"imageIds"`
	Visibility images.Visibility `json:"visibility"`
}

type deleteImageResponse struct {
	ImageID          string `json:"imageId"`
	ImageFileDeleted bool   `json:"imageFileDeleted"`
	ThumbnailDeleted bool   `json:"thumbnailDeleted"`
	CleanupPending   bool   `json:"cleanupPending"`
}

type conversionResponse struct {
	Image               imageResponse `json:"image"`
	OriginalFileDeleted bool          `json:"originalFileDeleted"`
	ThumbnailUpdated    bool          `json:"thumbnailUpdated"`
	CleanupPending      bool          `json:"cleanupPending"`
}

type batchOperationItem struct {
	ImageID        string `json:"imageId"`
	Status         string `json:"status"`
	CleanupPending bool   `json:"cleanupPending,omitempty"`
	ErrorCode      string `json:"errorCode,omitempty"`
}

type batchOperationResponse struct {
	Items []batchOperationItem `json:"items"`
}

func newImageHandler(
	service *images.Service,
	aliases *imagealias.Service,
	settingsService *settings.Service,
	filesystem *storage.Filesystem,
	authenticator *authenticator,
	logger *slog.Logger,
) *imageHandler {
	return &imageHandler{service: service, aliases: aliases, settings: settingsService, storage: filesystem, authenticator: authenticator, logger: logger}
}

func (h *imageHandler) upload(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authenticator.requireScope(w, r, apitoken.ScopeImagesUpload)
	if !ok {
		return
	}
	currentSettings, err := h.settings.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read upload settings.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, currentSettings.MaxUploadBytes+multipartOverheadBytes)
	if err := r.ParseMultipartForm(256 << 10); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Image exceeds the maximum upload size.")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_multipart", "A multipart image upload is required.")
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		writeError(w, r, http.StatusBadRequest, "single_file_required", "Exactly one image file is required.")
		return
	}
	visibility := currentSettings.DefaultVisibility
	if rawVisibility := strings.TrimSpace(r.FormValue("visibility")); rawVisibility != "" {
		visibility = images.Visibility(rawVisibility)
		if visibility != images.VisibilityPublic && visibility != images.VisibilityPrivate {
			writeError(w, r, http.StatusBadRequest, "invalid_visibility", "Visibility must be public or private.")
			return
		}
	}
	file, err := files[0].Open()
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_multipart", "Unable to open uploaded image.")
		return
	}
	defer file.Close()
	options := images.UploadOptions{Visibility: visibility, UploadedVia: "admin"}
	options.Limits = processor.Limits{
		MaxBytes: currentSettings.MaxUploadBytes, MaxTotalPixels: currentSettings.MaxTotalPixels,
	}
	options.Processing = processor.Options{
		CompressionEnabled:     currentSettings.CompressionEnabled,
		JPEGQuality:            currentSettings.JPEGQuality,
		WebPQuality:            currentSettings.WebPQuality,
		PNGCompressionLevel:    currentSettings.PNGCompressionLevel,
		ConversionEnabled:      currentSettings.ConversionEnabled,
		ConversionWebPQuality:  currentSettings.ConversionWebPQuality,
		ConversionWebPLossless: currentSettings.ConversionWebPLossless,
	}
	if principal.APIToken != nil {
		tokenID := principal.APIToken.TokenID
		options.UploadedVia = "api_token"
		options.UploadedByAPITokenID = &tokenID
	}
	uploaded, err := h.service.Upload(r.Context(), file, files[0].Filename, options, time.Now())
	if err != nil {
		h.writeUploadError(w, r, err)
		return
	}
	snapshot := maintenance.CaptureRuntime()
	h.logger.Info("image upload completed",
		"image_id", uploaded.ID,
		"source_bytes", uploaded.SourceSize,
		"stored_bytes", uploaded.StoredSize,
		"visibility", uploaded.Visibility,
		"uploaded_via", uploaded.UploadedVia,
		"go_heap_alloc_bytes", snapshot.HeapAllocBytes,
		"go_heap_sys_bytes", snapshot.HeapSysBytes,
		"goroutines", snapshot.Goroutines,
	)
	writeJSON(w, http.StatusCreated, toImageResponse(uploaded))
}

func (h *imageHandler) thumbnail(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, false); !ok {
		return
	}
	rawID := chi.URLParam(r, "imageID")
	id, err := uuid.Parse(rawID)
	if err != nil || id.String() != rawID {
		http.NotFound(w, r)
		return
	}
	file, err := h.storage.OpenThumbnail(rawID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, rawID+".jpg", info.ModTime(), file)
}

func (h *imageHandler) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, false); !ok {
		return
	}
	filter, err := parseImageListFilter(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_image_filter", "Image list filter is invalid.")
		return
	}
	page, err := h.service.Search(r.Context(), filter)
	if err != nil {
		if errors.Is(err, images.ErrInvalidListFilter) {
			writeError(w, r, http.StatusBadRequest, "invalid_image_filter", "Image list filter is invalid.")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to list images.")
		return
	}
	response := imageListResponse{Items: make([]imageResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, value := range page.Items {
		response.Items = append(response.Items, toImageResponse(value))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *imageHandler) detail(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, false); !ok {
		return
	}
	value, err := h.service.Get(r.Context(), chi.URLParam(r, "imageID"))
	if err != nil {
		if errors.Is(err, images.ErrImageNotFound) {
			writeError(w, r, http.StatusNotFound, "image_not_found", "Image was not found.")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read image details.")
		return
	}
	response := imageDetailResponse{imageResponse: toImageResponse(value), Aliases: []aliasResponse{}}
	if h.aliases != nil {
		aliases, err := h.aliases.ListByImage(r.Context(), value.ID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read image aliases.")
			return
		}
		response.Aliases = make([]aliasResponse, 0, len(aliases))
		for _, value := range aliases {
			response.Aliases = append(response.Aliases, toAliasResponse(value))
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *imageHandler) changeVisibility(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	rawID := chi.URLParam(r, "imageID")
	id, err := uuid.Parse(rawID)
	if err != nil || id.String() != rawID {
		writeError(w, r, http.StatusNotFound, "image_not_found", "Image was not found.")
		return
	}
	var request visibilityRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid visibility request.")
		return
	}
	updated, err := h.service.ChangeVisibility(r.Context(), rawID, request.Visibility)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to update image visibility.")
		return
	}
	if !updated {
		writeError(w, r, http.StatusNotFound, "image_not_found", "Image was not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *imageHandler) convertToWebP(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	currentSettings, err := h.settings.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read image processing settings.")
		return
	}
	result, err := h.service.ConvertToWebP(r.Context(), chi.URLParam(r, "imageID"), processor.Options{
		ConversionEnabled: true, ConversionWebPQuality: currentSettings.ConversionWebPQuality,
		ConversionWebPLossless: currentSettings.ConversionWebPLossless,
	})
	if err != nil {
		switch {
		case errors.Is(err, images.ErrImageNotFound):
			writeError(w, r, http.StatusNotFound, "image_not_found", "Image was not found.")
		case errors.Is(err, images.ErrConversionNotSupported):
			writeError(w, r, http.StatusConflict, "conversion_not_supported", err.Error())
		case errors.Is(err, images.ErrProcessingBusy):
			w.Header().Set("Retry-After", "1")
			writeError(w, r, http.StatusServiceUnavailable, "processing_busy", "Image processor is at capacity. Retry shortly.")
		case errors.Is(err, images.ErrProcessingUnavailable):
			writeError(w, r, http.StatusServiceUnavailable, "processing_unavailable", "Image byte processing is unavailable in this build.")
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to convert image to WebP.")
		}
		return
	}
	if result.CleanupPending {
		h.logger.Warn("image WebP conversion cleanup pending", "image_id", result.Image.ID, "original_file_error", result.OriginalFileError, "thumbnail_error", result.ThumbnailError)
	}
	writeJSON(w, http.StatusOK, conversionResponse{
		Image: toImageResponse(result.Image), OriginalFileDeleted: result.OriginalFileDeleted,
		ThumbnailUpdated: result.ThumbnailUpdated, CleanupPending: result.CleanupPending,
	})
}

func (h *imageHandler) delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireScope(w, r, apitoken.ScopeImagesDelete); !ok {
		return
	}
	result, err := h.service.Delete(r.Context(), chi.URLParam(r, "imageID"))
	if err != nil {
		if errors.Is(err, images.ErrImageNotFound) {
			writeError(w, r, http.StatusNotFound, "image_not_found", "Image was not found.")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to permanently delete image.")
		return
	}
	h.logDeleteResult(result)
	writeJSON(w, http.StatusOK, toDeleteImageResponse(result))
}

func (h *imageHandler) batchDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireScope(w, r, apitoken.ScopeImagesDelete); !ok {
		return
	}
	var request batchImageRequest
	if err := decodeJSON(w, r, &request); err != nil || len(request.ImageIDs) == 0 || len(request.ImageIDs) > 100 {
		writeError(w, r, http.StatusBadRequest, "invalid_batch", "Batch delete requires 1 to 100 image IDs.")
		return
	}
	ids, ok := uniqueImageIDs(request.ImageIDs)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "invalid_batch", "Batch image IDs must be unique UUIDs.")
		return
	}
	response := batchOperationResponse{Items: make([]batchOperationItem, 0, len(ids))}
	for _, id := range ids {
		result, err := h.service.Delete(r.Context(), id)
		item := batchOperationItem{ImageID: id, Status: "deleted"}
		switch {
		case errors.Is(err, images.ErrImageNotFound):
			item.Status = "not_found"
			item.ErrorCode = "image_not_found"
		case err != nil:
			item.Status = "error"
			item.ErrorCode = "delete_failed"
		case result.CleanupPending:
			item.Status = "cleanup_pending"
			item.CleanupPending = true
			h.logDeleteResult(result)
		default:
			h.logDeleteResult(result)
		}
		response.Items = append(response.Items, item)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *imageHandler) batchVisibility(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	var request batchVisibilityRequest
	if err := decodeJSON(w, r, &request); err != nil || len(request.ImageIDs) == 0 || len(request.ImageIDs) > 100 ||
		(request.Visibility != images.VisibilityPublic && request.Visibility != images.VisibilityPrivate) {
		writeError(w, r, http.StatusBadRequest, "invalid_batch", "Batch visibility requires 1 to 100 image IDs and a valid visibility.")
		return
	}
	ids, ok := uniqueImageIDs(request.ImageIDs)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "invalid_batch", "Batch image IDs must be unique UUIDs.")
		return
	}
	response := batchOperationResponse{Items: make([]batchOperationItem, 0, len(ids))}
	for _, id := range ids {
		updated, err := h.service.ChangeVisibility(r.Context(), id, request.Visibility)
		item := batchOperationItem{ImageID: id, Status: "updated"}
		if err != nil {
			item.Status = "error"
			item.ErrorCode = "visibility_failed"
		} else if !updated {
			item.Status = "not_found"
			item.ErrorCode = "image_not_found"
		}
		response.Items = append(response.Items, item)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *imageHandler) logDeleteResult(result images.DeleteResult) {
	if result.CleanupPending {
		h.logger.Warn("image delete cleanup pending",
			"image_id", result.ImageID,
			"image_file_deleted", result.ImageFileDeleted,
			"thumbnail_deleted", result.ThumbnailDeleted,
			"image_file_error", result.ImageCleanupError,
			"thumbnail_error", result.ThumbCleanupError,
		)
		return
	}
	h.logger.Info("image permanently deleted",
		"image_id", result.ImageID,
		"image_file_deleted", result.ImageFileDeleted,
		"thumbnail_deleted", result.ThumbnailDeleted,
	)
}

func toDeleteImageResponse(result images.DeleteResult) deleteImageResponse {
	return deleteImageResponse{
		ImageID: result.ImageID, ImageFileDeleted: result.ImageFileDeleted,
		ThumbnailDeleted: result.ThumbnailDeleted, CleanupPending: result.CleanupPending,
	}
}

func uniqueImageIDs(values []string) ([]string, bool) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id.String() != value {
			return nil, false
		}
		if _, exists := seen[value]; exists {
			return nil, false
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, true
}

func parseImageListFilter(r *http.Request) (images.ListFilter, error) {
	query := r.URL.Query()
	filter := images.ListFilter{Cursor: query.Get("cursor"), Query: query.Get("q"), Visibility: images.Visibility(query.Get("visibility")), UploadedVia: query.Get("uploadedVia")}
	var err error
	if filter.Limit, err = parseOptionalInt(query.Get("limit")); err != nil {
		return images.ListFilter{}, err
	}
	formats := map[string]string{"": "", "jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp", "gif": "image/gif"}
	mimeType, ok := formats[query.Get("format")]
	if !ok {
		return images.ListFilter{}, errors.New("invalid image format")
	}
	filter.MIMEType = mimeType
	if filter.CreatedFrom, err = parseOptionalTime(query.Get("createdFrom")); err != nil {
		return images.ListFilter{}, err
	}
	if filter.CreatedTo, err = parseOptionalTime(query.Get("createdTo")); err != nil {
		return images.ListFilter{}, err
	}
	if filter.MinStoredBytes, err = parseOptionalInt64(query.Get("minBytes")); err != nil {
		return images.ListFilter{}, err
	}
	if filter.MaxStoredBytes, err = parseOptionalInt64(query.Get("maxBytes")); err != nil {
		return images.ListFilter{}, err
	}
	for _, value := range []struct {
		raw         string
		destination *int
	}{
		{query.Get("minWidth"), &filter.MinWidth}, {query.Get("maxWidth"), &filter.MaxWidth},
		{query.Get("minHeight"), &filter.MinHeight}, {query.Get("maxHeight"), &filter.MaxHeight},
	} {
		if *value.destination, err = parseOptionalInt(value.raw); err != nil {
			return images.ListFilter{}, err
		}
	}
	return filter, nil
}

func parseOptionalInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func parseOptionalInt64(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func parseOptionalTime(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	value = value.UTC()
	return &value, nil
}

func (h *imageHandler) writeUploadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, images.ErrFileTooLarge):
		writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large", err.Error())
	case errors.Is(err, images.ErrInvalidImage), errors.Is(err, images.ErrUnsupportedFormat), errors.Is(err, images.ErrTooManyPixels):
		writeError(w, r, http.StatusBadRequest, "invalid_image", err.Error())
	case errors.Is(err, images.ErrProcessingBusy):
		w.Header().Set("Retry-After", "1")
		writeError(w, r, http.StatusServiceUnavailable, "processing_busy", "Image processor is at capacity. Retry shortly.")
	case errors.Is(err, images.ErrProcessingUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "processing_unavailable", "Image byte processing is unavailable in this build.")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to store image.")
	}
}

func toImageResponse(value images.Image) imageResponse {
	return imageResponse{
		ID:                value.ID,
		OriginalName:      value.OriginalName,
		MIMEType:          value.MIMEType,
		Extension:         value.Extension,
		Width:             value.Width,
		Height:            value.Height,
		SourceSize:        value.SourceSize,
		StoredSize:        value.StoredSize,
		SourceSHA256:      hex.EncodeToString(value.SourceSHA256[:]),
		StoredSHA256:      hex.EncodeToString(value.StoredSHA256[:]),
		Visibility:        value.Visibility,
		UploadedVia:       value.UploadedVia,
		StandardURL:       "/image/" + value.ID,
		ThumbnailURL:      "/api/v1/images/" + value.ID + "/thumbnail",
		ProcessingSummary: json.RawMessage(value.ProcessingSummary),
		CreatedAt:         value.CreatedAt,
	}
}
