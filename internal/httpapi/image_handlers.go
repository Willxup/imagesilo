package httpapi

import (
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Willxup/imagesilo/internal/auth"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/maintenance"
)

const maxMultipartRequestBytes = 22 << 20

type imageHandler struct {
	service *images.Service
	auth    *authHandler
	logger  *slog.Logger
}

type imageResponse struct {
	ID           string            `json:"id"`
	OriginalName string            `json:"originalName"`
	MIMEType     string            `json:"mimeType"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	SourceSize   int64             `json:"sourceSize"`
	StoredSize   int64             `json:"storedSize"`
	SourceSHA256 string            `json:"sourceSha256"`
	StoredSHA256 string            `json:"storedSha256"`
	Visibility   images.Visibility `json:"visibility"`
	StandardURL  string            `json:"standardUrl"`
	CreatedAt    time.Time         `json:"createdAt"`
}

type imageListResponse struct {
	Items []imageResponse `json:"items"`
}

func newImageHandler(service *images.Service, authService *auth.Service, logger *slog.Logger) *imageHandler {
	return &imageHandler{service: service, auth: newAuthHandler(authService, true), logger: logger}
}

func (h *imageHandler) upload(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth.authenticateRequest(r); !ok {
		writeError(w, r, http.StatusUnauthorized, "authentication_required", "Administrator session is required.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartRequestBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_multipart", "A multipart image upload is required.")
		return
	}

	var uploaded *images.Image
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if uploaded == nil {
				writeError(w, r, http.StatusBadRequest, "invalid_multipart", "Unable to read the multipart upload.")
				return
			}
			break
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		value, uploadErr := h.service.UploadJPEG(r.Context(), part, part.FileName(), time.Now())
		part.Close()
		if uploadErr != nil {
			h.writeUploadError(w, r, uploadErr)
			return
		}
		uploaded = &value
		break
	}
	if uploaded == nil {
		writeError(w, r, http.StatusBadRequest, "file_required", "The file field is required.")
		return
	}
	snapshot := maintenance.CaptureRuntime()
	h.logger.Info("image upload completed",
		"image_id", uploaded.ID,
		"source_bytes", uploaded.SourceSize,
		"stored_bytes", uploaded.StoredSize,
		"go_heap_alloc_bytes", snapshot.HeapAllocBytes,
		"go_heap_sys_bytes", snapshot.HeapSysBytes,
		"goroutines", snapshot.Goroutines,
	)
	writeJSON(w, http.StatusCreated, toImageResponse(*uploaded))
}

func (h *imageHandler) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth.authenticateRequest(r); !ok {
		writeError(w, r, http.StatusUnauthorized, "authentication_required", "Administrator session is required.")
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

func (h *imageHandler) writeUploadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, images.ErrFileTooLarge):
		writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large", err.Error())
	case errors.Is(err, images.ErrInvalidJPEG), errors.Is(err, images.ErrTooManyPixels):
		writeError(w, r, http.StatusBadRequest, "invalid_image", err.Error())
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to store image.")
	}
}

func toImageResponse(value images.Image) imageResponse {
	return imageResponse{
		ID:           value.ID,
		OriginalName: value.OriginalName,
		MIMEType:     value.MIMEType,
		Width:        value.Width,
		Height:       value.Height,
		SourceSize:   value.SourceSize,
		StoredSize:   value.StoredSize,
		SourceSHA256: hex.EncodeToString(value.SourceSHA256[:]),
		StoredSHA256: hex.EncodeToString(value.StoredSHA256[:]),
		Visibility:   value.Visibility,
		StandardURL:  "/image/" + value.ID,
		CreatedAt:    value.CreatedAt,
	}
}
