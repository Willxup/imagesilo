package httpapi

import (
	"net/http"

	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/settings"
)

type systemHandler struct {
	settings              *settings.Service
	authenticator         *authenticator
	processingConcurrency int
	deliveryConcurrency   int
}

type systemResponse struct {
	ProcessingConcurrency int      `json:"processingConcurrency"`
	DeliveryConcurrency   int      `json:"deliveryConcurrency"`
	MaxBatchCount         int      `json:"maxBatchCount"`
	MaxUploadBytes        int64    `json:"maxUploadBytes"`
	MaxTotalPixels        int64    `json:"maxTotalPixels"`
	SupportedFormats      []string `json:"supportedFormats"`
	VIPSVersion           string   `json:"vipsVersion"`
}

func newSystemHandler(service *settings.Service, authenticator *authenticator, processingConcurrency, deliveryConcurrency int) *systemHandler {
	return &systemHandler{settings: service, authenticator: authenticator, processingConcurrency: processingConcurrency, deliveryConcurrency: deliveryConcurrency}
}

func (h *systemHandler) get(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticator.requireSession(w, r, false); !ok {
		return
	}
	value, err := h.settings.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to read system limits.")
		return
	}
	writeJSON(w, http.StatusOK, systemResponse{
		ProcessingConcurrency: h.processingConcurrency,
		DeliveryConcurrency:   h.deliveryConcurrency,
		MaxBatchCount:         value.MaxBatchCount,
		MaxUploadBytes:        value.MaxUploadBytes,
		MaxTotalPixels:        value.MaxTotalPixels,
		SupportedFormats:      []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
		VIPSVersion:           processor.VIPSVersion(),
	})
}
