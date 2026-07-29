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
	StandardURL       string            `json:"standardUrl"`
	ThumbnailURL      string            `json:"thumbnailUrl"`
	ProcessingSummary json.RawMessage   `json:"processingSummary"`
	CreatedAt         time.Time         `json:"createdAt"`
}

type imageListResponse struct {
	Items []imageResponse `json:"items"`
}

type visibilityRequest struct {
	Visibility images.Visibility `json:"visibility"`
}

func newImageHandler(
	service *images.Service,
	settingsService *settings.Service,
	filesystem *storage.Filesystem,
	authenticator *authenticator,
	logger *slog.Logger,
) *imageHandler {
	return &imageHandler{service: service, settings: settingsService, storage: filesystem, authenticator: authenticator, logger: logger}
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
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	values, err := h.service.List(r.Context(), limit)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to list images.")
		return
	}
	response := imageListResponse{Items: make([]imageResponse, 0, len(values))}
	for _, value := range values {
		response.Items = append(response.Items, toImageResponse(value))
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
		StandardURL:       "/image/" + value.ID,
		ThumbnailURL:      "/api/v1/images/" + value.ID + "/thumbnail",
		ProcessingSummary: json.RawMessage(value.ProcessingSummary),
		CreatedAt:         value.CreatedAt,
	}
}
