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
	index   *delivery.Index
	storage *storage.Filesystem
}

func newDeliveryHandler(index *delivery.Index, filesystem *storage.Filesystem) *deliveryHandler {
	return &deliveryHandler{index: index, storage: filesystem}
}

func (h *deliveryHandler) serve(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "imageID")
	id, err := uuid.Parse(rawID)
	if err != nil || id.String() != rawID {
		http.NotFound(w, r)
		return
	}
	target, ok := h.index.Get(rawID)
	if !ok || target.Visibility != "public" {
		http.NotFound(w, r)
		return
	}
	file, err := h.storage.Open(target.StorageKey)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", target.MIMEType)
	w.Header().Set("ETag", target.ETag)
	w.Header().Set("Cache-Control", "public, no-cache")
	if disposition := mime.FormatMediaType("inline", map[string]string{"filename": target.OriginalName}); disposition != "" {
		w.Header().Set("Content-Disposition", disposition)
	}
	http.ServeContent(w, r, target.OriginalName, target.LastModified, file)
}
