package httpapi

import (
	"encoding/hex"
	"errors"
	"io"
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
	lease, ok := h.service.TryAcquireProcessing()
	if !ok {
		w.Header().Set("Retry-After", "1")
		writeError(w, r, http.StatusServiceUnavailable, "processing_busy", "Image processor is at capacity. Retry shortly.")
		return
	}
	defer lease.Release()
	multipartReader, err := r.MultipartReader()
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_multipart", "A multipart image import is required.")
		return
	}
	var staged *importer.StagedImport
	defer func() { h.service.Discard(staged) }()
	var originalName, aliasPath, visibilityValue string
	aliasSeen := false
	visibilitySeen := false
	for {
		part, partErr := multipartReader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if partErr != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(partErr, &maxBytesError) {
				writeError(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Image exceeds the maximum upload size.")
			} else {
				writeError(w, r, http.StatusBadRequest, "invalid_multipart", "Unable to read multipart import.")
			}
			return
		}
		switch part.FormName() {
		case "file":
			if staged != nil || part.FileName() == "" {
				part.Close()
				writeError(w, r, http.StatusBadRequest, "single_import_required", "Exactly one image file and one alias path are required.")
				return
			}
			originalName = part.FileName()
			staged, err = h.service.Stage(part, currentSettings.MaxUploadBytes)
			part.Close()
			if err != nil {
				h.writeServiceError(w, r, err)
				return
			}
		case "alias":
			if aliasSeen || part.FileName() != "" {
				part.Close()
				writeError(w, r, http.StatusBadRequest, "single_import_required", "Exactly one image file and one alias path are required.")
				return
			}
			value, readErr := io.ReadAll(io.LimitReader(part, 2049))
			part.Close()
			if readErr != nil || len(value) > 2048 {
				writeError(w, r, http.StatusBadRequest, "invalid_alias", "Alias path is too long.")
				return
			}
			aliasSeen = true
			aliasPath = string(value)
		case "visibility":
			if visibilitySeen || part.FileName() != "" {
				part.Close()
				writeError(w, r, http.StatusBadRequest, "invalid_visibility", "Visibility must be supplied once as public or private.")
				return
			}
			value, readErr := io.ReadAll(io.LimitReader(part, 65))
			part.Close()
			if readErr != nil || len(value) > 64 {
				writeError(w, r, http.StatusBadRequest, "invalid_visibility", "Visibility must be public or private.")
				return
			}
			visibilitySeen = true
			visibilityValue = strings.TrimSpace(string(value))
		default:
			part.Close()
			writeError(w, r, http.StatusBadRequest, "invalid_multipart", "Multipart import contains an unsupported field.")
			return
		}
	}
	if staged == nil || !aliasSeen {
		writeError(w, r, http.StatusBadRequest, "single_import_required", "Exactly one image file and one alias path are required.")
		return
	}
	visibility := currentSettings.DefaultVisibility
	if visibilityValue != "" {
		visibility = images.Visibility(visibilityValue)
		if visibility != images.VisibilityPublic && visibility != images.VisibilityPrivate {
			writeError(w, r, http.StatusBadRequest, "invalid_visibility", "Visibility must be public or private.")
			return
		}
	}
	options := importer.Options{
		Visibility: visibility,
		Limits:     processor.Limits{MaxBytes: currentSettings.MaxUploadBytes, MaxTotalPixels: currentSettings.MaxTotalPixels},
	}
	if principal.APIToken != nil {
		tokenID := principal.APIToken.TokenID
		options.UploadedByAPITokenID = &tokenID
	}
	result, err := h.service.ImportStaged(r.Context(), staged, originalName, aliasPath, options, time.Now(), lease)
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
