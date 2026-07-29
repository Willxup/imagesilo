package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/Willxup/imagesilo/internal/settings"
	"github.com/Willxup/imagesilo/internal/webui"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type healthResponse struct {
	Status string `json:"status"`
}

type Dependencies struct {
	DB                    *sql.DB
	Logger                *slog.Logger
	Auth                  *auth.Service
	APITokens             *apitoken.Service
	Images                *images.Service
	Settings              *settings.Service
	DeliveryIndex         *delivery.Index
	Storage               *storage.Filesystem
	CookieSecure          bool
	ProcessingConcurrency int
	UI                    *webui.UI
}

func NewRouter(dependencies Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(securityHeaders)
	authenticator := newAuthenticator(dependencies.Auth, dependencies.APITokens)

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
		authHandler := newAuthHandler(dependencies.Auth, authenticator, dependencies.CookieSecure)
		router.Post("/api/v1/auth/login", authHandler.login)
		router.Get("/api/v1/auth/session", authHandler.current)
		router.Post("/api/v1/auth/logout", authHandler.logout)
		router.Post("/api/v1/auth/password", authHandler.changePassword)
	}
	if dependencies.APITokens != nil && dependencies.Auth != nil {
		tokenHandler := newAPITokenHandler(dependencies.APITokens, authenticator)
		router.Get("/api/v1/api-tokens", tokenHandler.list)
		router.Post("/api/v1/api-tokens", tokenHandler.create)
		router.Delete("/api/v1/api-tokens/{tokenID}", tokenHandler.revoke)
	}
	if dependencies.Images != nil && dependencies.Auth != nil && dependencies.Settings != nil && dependencies.Storage != nil {
		imageHandler := newImageHandler(dependencies.Images, dependencies.Settings, dependencies.Storage, authenticator, dependencies.Logger)
		router.Get("/api/v1/images", imageHandler.list)
		router.Post("/api/v1/images", imageHandler.upload)
		router.Get("/api/v1/images/{imageID}/thumbnail", imageHandler.thumbnail)
		router.Patch("/api/v1/images/{imageID}/visibility", imageHandler.changeVisibility)
	}
	if dependencies.Settings != nil && dependencies.Auth != nil {
		settingsHandler := newSettingsHandler(dependencies.Settings, authenticator)
		router.Get("/api/v1/settings", settingsHandler.get)
		router.Patch("/api/v1/settings/default-visibility", settingsHandler.updateDefaultVisibility)
		router.Patch("/api/v1/settings/processing", settingsHandler.updateProcessing)
		systemHandler := newSystemHandler(dependencies.Settings, authenticator, dependencies.ProcessingConcurrency)
		router.Get("/api/v1/system", systemHandler.get)
	}
	if dependencies.DeliveryIndex != nil && dependencies.Storage != nil {
		deliveryHandler := newDeliveryHandler(dependencies.DeliveryIndex, dependencies.Storage, authenticator)
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
