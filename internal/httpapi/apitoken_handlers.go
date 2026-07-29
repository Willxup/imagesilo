package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/go-chi/chi/v5"
)

type apiTokenHandler struct {
	service       *apitoken.Service
	authenticator *authenticator
}

type createAPITokenRequest struct {
	Name      string           `json:"name"`
	Scopes    []apitoken.Scope `json:"scopes"`
	ExpiresAt *time.Time       `json:"expiresAt"`
}

type apiTokenResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	TokenPrefix string           `json:"tokenPrefix"`
	Scopes      []apitoken.Scope `json:"scopes"`
	ExpiresAt   *time.Time       `json:"expiresAt"`
	Status      string           `json:"status"`
	CreatedAt   time.Time        `json:"createdAt"`
	Token       string           `json:"token,omitempty"`
}

type apiTokenListResponse struct {
	Items []apiTokenResponse `json:"items"`
}

func newAPITokenHandler(service *apitoken.Service, authenticator *authenticator) *apiTokenHandler {
	return &apiTokenHandler{service: service, authenticator: authenticator}
}

func (h *apiTokenHandler) create(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	var request createAPITokenRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid API token request.")
		return
	}
	token, plaintext, err := h.service.Create(r.Context(), request.Name, request.Scopes, request.ExpiresAt, time.Now())
	if err != nil {
		if errors.Is(err, apitoken.ErrInvalidName) || errors.Is(err, apitoken.ErrInvalidScope) || errors.Is(err, apitoken.ErrInvalidExpiration) {
			writeError(w, r, http.StatusBadRequest, "invalid_api_token", err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to create API token.")
		return
	}
	response := toAPITokenResponse(token, time.Now())
	response.Token = plaintext
	writeJSON(w, http.StatusCreated, response)
}

func (h *apiTokenHandler) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, false); !ok {
		return
	}
	tokens, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to list API tokens.")
		return
	}
	response := apiTokenListResponse{Items: make([]apiTokenResponse, 0, len(tokens))}
	now := time.Now()
	for _, token := range tokens {
		response.Items = append(response.Items, toAPITokenResponse(token, now))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *apiTokenHandler) revoke(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, true); !ok {
		return
	}
	if err := h.service.Revoke(r.Context(), chi.URLParam(r, "tokenID")); err != nil {
		if errors.Is(err, apitoken.ErrTokenNotFound) {
			writeError(w, r, http.StatusNotFound, "api_token_not_found", "API token was not found or is already revoked.")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to revoke API token.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toAPITokenResponse(token apitoken.Token, now time.Time) apiTokenResponse {
	status := token.Status
	if status == "active" && token.ExpiresAt != nil && !token.ExpiresAt.After(now) {
		status = "expired"
	}
	return apiTokenResponse{
		ID: token.ID, Name: token.Name, TokenPrefix: token.TokenPrefix, Scopes: token.Scopes,
		ExpiresAt: token.ExpiresAt, Status: status, CreatedAt: token.CreatedAt,
	}
}
