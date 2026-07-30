package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/Willxup/imagesilo/internal/auth"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/setup"
)

type setupHandler struct {
	service *setup.Service
	auth    *authHandler
}

type setupStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type setupRequest struct {
	DisplayName           string            `json:"displayName"`
	Email                 string            `json:"email"`
	Password              string            `json:"password"`
	DefaultVisibility     images.Visibility `json:"defaultVisibility"`
	CompressionEnabled    bool              `json:"compressionEnabled"`
	JPEGQuality           int               `json:"jpegQuality"`
	WebPQuality           int               `json:"webpQuality"`
	PNGCompressionLevel   int               `json:"pngCompressionLevel"`
	ConversionEnabled     bool              `json:"conversionEnabled"`
	ConversionWebPQuality int               `json:"conversionWebpQuality"`
	ConversionLossless    bool              `json:"conversionWebpLossless"`
}

func newSetupHandler(service *setup.Service, authHandler *authHandler) *setupHandler {
	return &setupHandler{service: service, auth: authHandler}
}

func (h *setupHandler) status(w http.ResponseWriter, r *http.Request) {
	initialized, err := h.service.Initialized(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read setup status.")
		return
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{Initialized: initialized})
}

func (h *setupHandler) initialize(w http.ResponseWriter, r *http.Request) {
	var request setupRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid setup request.")
		return
	}
	_, err := h.service.Initialize(r.Context(), setup.Request{
		DisplayName: request.DisplayName, Email: request.Email, Password: request.Password,
		DefaultVisibility: request.DefaultVisibility, CompressionEnabled: request.CompressionEnabled,
		JPEGQuality: request.JPEGQuality, WebPQuality: request.WebPQuality, PNGCompressionLevel: request.PNGCompressionLevel,
		ConversionEnabled: request.ConversionEnabled, ConversionWebPQuality: request.ConversionWebPQuality,
		ConversionLossless: request.ConversionLossless,
	}, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, setup.ErrAlreadyInitialized):
			writeError(w, r, http.StatusConflict, "already_initialized", err.Error())
		case errors.Is(err, setup.ErrInvalidSettings), errors.Is(err, auth.ErrInvalidDisplayName), errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrPasswordTooShort):
			writeError(w, r, http.StatusBadRequest, "invalid_setup", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to initialize ImageSilo.")
		}
		return
	}
	identity, token, csrfToken, err := h.auth.service.Login(r.Context(), request.Email, request.Password, time.Now())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "ImageSilo was initialized, but the administrator session could not be created.")
		return
	}
	h.auth.setSessionCookies(w, token, csrfToken, identity.ExpiresAt)
	writeJSON(w, http.StatusCreated, toSessionResponse(identity, csrfToken))
}
