package httpapi

import (
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	imagealias "github.com/Willxup/imagesilo/internal/alias"
	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/importer"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/settings"
)

type importHandler struct {
	service       *importer.Service
	settings      *settings.Service
	authenticator *authenticator
}

type importResponse struct {
	ImageID     string        `json:"imageId"`
	StandardURL string        `json:"standardUrl"`
	SHA256      string        `json:"sha256"`
	Alias       aliasResponse `json:"alias"`
}

func newImportHandler(service *importer.Service, settingsService *settings.Service, authenticator *authenticator) *importHandler {
	return &importHandler{service: service, settings: settingsService, authenticator: authenticator}
}

func (h *importHandler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authenticator.requireScopes(w, r, apitoken.ScopeImagesUpload, apitoken.ScopeAliasesWrite)
	if !ok {
		return
	}
	currentSettings, err := h.settings.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read import settings.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, currentSettings.MaxUploadBytes+multipartOverheadBytes)
	if err := r.ParseMultipartForm(256 << 10); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Image exceeds the maximum upload size.")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_multipart", "A multipart image import is required.")
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["file"]
	aliases := r.MultipartForm.Value["alias"]
	visibilities := r.MultipartForm.Value["visibility"]
	if len(files) != 1 || len(aliases) != 1 || len(visibilities) > 1 {
		writeError(w, r, http.StatusBadRequest, "single_import_required", "Exactly one image file and one alias path are required.")
		return
	}
	visibility := currentSettings.DefaultVisibility
	if len(visibilities) == 1 && strings.TrimSpace(visibilities[0]) != "" {
		visibility = images.Visibility(strings.TrimSpace(visibilities[0]))
		if visibility != images.VisibilityPublic && visibility != images.VisibilityPrivate {
			writeError(w, r, http.StatusBadRequest, "invalid_visibility", "Visibility must be public or private.")
			return
		}
	}
	file, err := files[0].Open()
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_multipart", "Unable to open imported image.")
		return
	}
	defer file.Close()
	options := importer.Options{
		Visibility: visibility,
		Limits:     processor.Limits{MaxBytes: currentSettings.MaxUploadBytes, MaxTotalPixels: currentSettings.MaxTotalPixels},
	}
	if principal.APIToken != nil {
		tokenID := principal.APIToken.TokenID
		options.UploadedByAPITokenID = &tokenID
	}
	result, err := h.service.Import(r.Context(), file, files[0].Filename, aliases[0], options, time.Now())
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, importResponse{
		ImageID: result.Image.ID, StandardURL: "/image/" + result.Image.ID,
		SHA256: hex.EncodeToString(result.Image.StoredSHA256[:]), Alias: toAliasResponse(result.Alias),
	})
}

func (h *importHandler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, delivery.ErrInvalidAliasPath), errors.Is(err, delivery.ErrReservedAliasPath):
		writeError(w, r, http.StatusBadRequest, "invalid_alias", err.Error())
	case errors.Is(err, imagealias.ErrAliasConflict):
		writeError(w, r, http.StatusConflict, "alias_conflict", "Alias path already exists.")
	case errors.Is(err, images.ErrFileTooLarge):
		writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large", err.Error())
	case errors.Is(err, images.ErrInvalidImage), errors.Is(err, images.ErrUnsupportedFormat), errors.Is(err, images.ErrTooManyPixels):
		writeError(w, r, http.StatusBadRequest, "invalid_image", err.Error())
	case errors.Is(err, images.ErrProcessingBusy):
		w.Header().Set("Retry-After", "1")
		writeError(w, r, http.StatusServiceUnavailable, "processing_busy", "Image processor is at capacity. Retry shortly.")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to import image.")
	}
}
