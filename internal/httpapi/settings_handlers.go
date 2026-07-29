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
	DefaultVisibility images.Visibility `json:"defaultVisibility"`
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
	writeJSON(w, http.StatusOK, settingsResponse{DefaultVisibility: value.DefaultVisibility})
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
	writeJSON(w, http.StatusOK, settingsResponse{DefaultVisibility: request.DefaultVisibility})
}
