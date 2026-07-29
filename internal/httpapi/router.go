package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/Willxup/imagesilo/internal/webui"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type healthResponse struct {
	Status string `json:"status"`
}

type Dependencies struct {
	DB            *sql.DB
	Logger        *slog.Logger
	Auth          *auth.Service
	Images        *images.Service
	DeliveryIndex *delivery.Index
	Storage       *storage.Filesystem
	CookieSecure  bool
	UI            *webui.UI
}

func NewRouter(dependencies Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(securityHeaders)

	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := dependencies.DB.PingContext(r.Context()); err != nil {
			dependencies.Logger.Error("readiness database check failed", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ready"})
	})

	if dependencies.Auth != nil {
		authHandler := newAuthHandler(dependencies.Auth, dependencies.CookieSecure)
		router.Post("/api/v1/auth/login", authHandler.login)
		router.Get("/api/v1/auth/session", authHandler.current)
		router.Post("/api/v1/auth/logout", authHandler.logout)
	}
	if dependencies.Images != nil && dependencies.Auth != nil {
		imageHandler := newImageHandler(dependencies.Images, dependencies.Auth, dependencies.Logger)
		router.Get("/api/v1/images", imageHandler.list)
		router.Post("/api/v1/images", imageHandler.upload)
	}
	if dependencies.DeliveryIndex != nil && dependencies.Storage != nil {
		deliveryHandler := newDeliveryHandler(dependencies.DeliveryIndex, dependencies.Storage)
		router.Get("/image/{imageID}", deliveryHandler.serve)
	}
	if dependencies.UI != nil {
		router.Handle("/assets/*", dependencies.UI.Assets())
		router.Get("/admin", dependencies.UI.ServeIndex)
		router.Get("/admin/*", dependencies.UI.ServeIndex)
	}
	return router
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
