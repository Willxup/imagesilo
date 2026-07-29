package httpapi

import (
	"errors"
	"net/http"
	"time"

	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/settings"
)

type settingsHandler struct {
	service       *settings.Service
	authenticator *authenticator
}

type defaultVisibilityRequest struct {
	DefaultVisibility images.Visibility `json:"defaultVisibility"`
}

type settingsResponse struct {
	DefaultVisibility      images.Visibility `json:"defaultVisibility"`
	CompressionEnabled     bool              `json:"compressionEnabled"`
	JPEGQuality            int               `json:"jpegQuality"`
	WebPQuality            int               `json:"webpQuality"`
	PNGCompressionLevel    int               `json:"pngCompressionLevel"`
	ConversionEnabled      bool              `json:"conversionEnabled"`
	ConversionWebPQuality  int               `json:"conversionWebpQuality"`
	ConversionWebPLossless bool              `json:"conversionWebpLossless"`
}

type processingSettingsRequest struct {
	CompressionEnabled     bool `json:"compressionEnabled"`
	JPEGQuality            int  `json:"jpegQuality"`
	WebPQuality            int  `json:"webpQuality"`
	PNGCompressionLevel    int  `json:"pngCompressionLevel"`
	ConversionEnabled      bool `json:"conversionEnabled"`
	ConversionWebPQuality  int  `json:"conversionWebpQuality"`
	ConversionWebPLossless bool `json:"conversionWebpLossless"`
}

func newSettingsHandler(service *settings.Service, authenticator *authenticator) *settingsHandler {
	return &settingsHandler{service: service, authenticator: authenticator}
}

func (h *settingsHandler) get(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, false); !ok {
		return
	}
	value, err := h.service.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read settings.")
		return
	}
	writeJSON(w, http.StatusOK, toSettingsResponse(value))
}

func (h *settingsHandler) updateDefaultVisibility(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	var request defaultVisibilityRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid settings request.")
		return
	}
	if err := h.service.UpdateDefaultVisibility(r.Context(), request.DefaultVisibility, time.Now()); err != nil {
		if errors.Is(err, settings.ErrInvalidVisibility) {
			writeError(w, r, http.StatusBadRequest, "invalid_visibility", err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to update default visibility.")
		return
	}
	value, err := h.service.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read updated settings.")
		return
	}
	writeJSON(w, http.StatusOK, toSettingsResponse(value))
}

func (h *settingsHandler) updateProcessing(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	var request processingSettingsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid image processing settings request.")
		return
	}
	value := settings.Settings{
		CompressionEnabled:     request.CompressionEnabled,
		JPEGQuality:            request.JPEGQuality,
		WebPQuality:            request.WebPQuality,
		PNGCompressionLevel:    request.PNGCompressionLevel,
		ConversionEnabled:      request.ConversionEnabled,
		ConversionWebPQuality:  request.ConversionWebPQuality,
		ConversionWebPLossless: request.ConversionWebPLossless,
	}
	if err := h.service.UpdateProcessing(r.Context(), value, time.Now()); err != nil {
		if errors.Is(err, settings.ErrInvalidProcessing) {
			writeError(w, r, http.StatusBadRequest, "invalid_processing_settings", err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to update image processing settings.")
		return
	}
	updated, err := h.service.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read updated settings.")
		return
	}
	writeJSON(w, http.StatusOK, toSettingsResponse(updated))
}

func toSettingsResponse(value settings.Settings) settingsResponse {
	return settingsResponse{
		DefaultVisibility:      value.DefaultVisibility,
		CompressionEnabled:     value.CompressionEnabled,
		JPEGQuality:            value.JPEGQuality,
		WebPQuality:            value.WebPQuality,
		PNGCompressionLevel:    value.PNGCompressionLevel,
		ConversionEnabled:      value.ConversionEnabled,
		ConversionWebPQuality:  value.ConversionWebPQuality,
		ConversionWebPLossless: value.ConversionWebPLossless,
	}
}
