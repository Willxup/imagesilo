package httpapi

import (
	"mime"
	"net/http"

	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type deliveryHandler struct {
	index         *delivery.Index
	storage       *storage.Filesystem
	authenticator *authenticator
}

func newDeliveryHandler(index *delivery.Index, filesystem *storage.Filesystem, authenticator *authenticator) *deliveryHandler {
	return &deliveryHandler{index: index, storage: filesystem, authenticator: authenticator}
}

func (h *deliveryHandler) serve(w http.ResponseWriter, r *http.Request) {
	if h.rejectURLToken(w, r) {
		return
	}
	rawID := chi.URLParam(r, "imageID")
	id, err := uuid.Parse(rawID)
	if err != nil || id.String() != rawID {
		h.serveAlias(w, r)
		return
	}
	target, ok := h.index.Get(rawID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.serveTarget(w, r, target)
}

func (h *deliveryHandler) serveAlias(w http.ResponseWriter, r *http.Request) {
	if h.rejectURLToken(w, r) {
		return
	}
	path, err := delivery.NormalizeAliasPath(r.URL.EscapedPath())
	if err != nil {
		http.NotFound(w, r)
		return
	}
	target, ok := h.index.GetAlias(path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.serveTarget(w, r, target)
}

func (h *deliveryHandler) rejectURLToken(w http.ResponseWriter, r *http.Request) bool {
	for _, name := range []string{"token", "key", "api_key", "access_token"} {
		if _, present := r.URL.Query()[name]; present {
			writeError(w, r, http.StatusBadRequest, "url_token_forbidden", "Authentication tokens are not accepted in image URLs.")
			return true
		}
	}
	return false
}

func (h *deliveryHandler) serveTarget(w http.ResponseWriter, r *http.Request, target delivery.Target) {
	if target.Visibility == "private" {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Vary", "Authorization, Cookie")
		allowed, insufficientScope := h.authenticator.privateImageAccess(r)
		if !allowed {
			w.Header().Set("WWW-Authenticate", `Bearer realm="imagesilo"`)
			if insufficientScope {
				writeError(w, r, http.StatusForbidden, "insufficient_scope", "Bearer token does not have images:read_private scope.")
			} else {
				writeError(w, r, http.StatusUnauthorized, "authentication_required", "Private image authentication is required.")
			}
			return
		}
	}
	file, err := h.storage.Open(target.StorageKey)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", target.MIMEType)
	w.Header().Set("ETag", target.ETag)
	if target.Visibility != "private" {
		w.Header().Set("Cache-Control", "public, no-cache")
	}
	if disposition := mime.FormatMediaType("inline", map[string]string{"filename": target.OriginalName}); disposition != "" {
		w.Header().Set("Content-Disposition", disposition)
	}
	http.ServeContent(w, r, target.OriginalName, target.LastModified, file)
}
