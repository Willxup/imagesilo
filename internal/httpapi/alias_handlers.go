package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	imagealias "github.com/Willxup/imagesilo/internal/alias"
	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/go-chi/chi/v5"
)

type aliasHandler struct {
	service       *imagealias.Service
	authenticator *authenticator
}

type createAliasRequest struct {
	Path    string `json:"path"`
	ImageID string `json:"imageId"`
	Source  string `json:"source"`
}

type aliasResponse struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	ImageID   string    `json:"imageId"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

type aliasListResponse struct {
	Items []aliasResponse `json:"items"`
}

func newAliasHandler(service *imagealias.Service, authenticator *authenticator) *aliasHandler {
	return &aliasHandler{service: service, authenticator: authenticator}
}

func (h *aliasHandler) create(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireScope(w, r, apitoken.ScopeAliasesWrite); !ok {
		return
	}
	var request createAliasRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid alias request.")
		return
	}
	value, err := h.service.Create(r.Context(), request.Path, request.ImageID, request.Source, time.Now())
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAliasResponse(value))
}

func (h *aliasHandler) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireScope(w, r, apitoken.ScopeAliasesWrite); !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	values, err := h.service.List(r.Context(), limit)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to list aliases.")
		return
	}
	response := aliasListResponse{Items: make([]aliasResponse, 0, len(values))}
	for _, value := range values {
		response.Items = append(response.Items, toAliasResponse(value))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *aliasHandler) resolve(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireScope(w, r, apitoken.ScopeAliasesWrite); !ok {
		return
	}
	value, err := h.service.Resolve(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAliasResponse(value))
}

func (h *aliasHandler) delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireScope(w, r, apitoken.ScopeAliasesWrite); !ok {
		return
	}
	if err := h.service.Delete(r.Context(), chi.URLParam(r, "aliasID")); err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *aliasHandler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, delivery.ErrInvalidAliasPath), errors.Is(err, delivery.ErrReservedAliasPath),
		errors.Is(err, imagealias.ErrInvalidImage), errors.Is(err, imagealias.ErrInvalidSource):
		writeError(w, r, http.StatusBadRequest, "invalid_alias", err.Error())
	case errors.Is(err, imagealias.ErrAliasConflict):
		writeError(w, r, http.StatusConflict, "alias_conflict", "Alias path already exists.")
	case errors.Is(err, imagealias.ErrAliasNotFound):
		writeError(w, r, http.StatusNotFound, "alias_not_found", "Alias was not found.")
	case errors.Is(err, imagealias.ErrImageNotFound):
		writeError(w, r, http.StatusNotFound, "image_not_found", "Target image was not found.")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to update aliases.")
	}
}

func toAliasResponse(value imagealias.Alias) aliasResponse {
	return aliasResponse{
		ID: value.ID, Path: value.Path, ImageID: value.ImageID,
		Source: value.Source, CreatedAt: value.CreatedAt,
	}
}
