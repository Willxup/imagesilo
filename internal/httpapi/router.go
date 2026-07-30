package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"

	imagealias "github.com/Willxup/imagesilo/internal/alias"
	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/importer"
	"github.com/Willxup/imagesilo/internal/maintenance"
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
	Aliases               *imagealias.Service
	Images                *images.Service
	Importer              *importer.Service
	Settings              *settings.Service
	Maintenance           *maintenance.Service
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
	if dependencies.Aliases != nil && dependencies.Auth != nil {
		aliasHandler := newAliasHandler(dependencies.Aliases, authenticator)
		router.Get("/api/v1/aliases", aliasHandler.list)
		router.Get("/api/v1/aliases/resolve", aliasHandler.resolve)
		router.Post("/api/v1/aliases", aliasHandler.create)
		router.Delete("/api/v1/aliases/{aliasID}", aliasHandler.delete)
	}
	if dependencies.Images != nil && dependencies.Auth != nil && dependencies.Settings != nil && dependencies.Storage != nil {
		imageHandler := newImageHandler(dependencies.Images, dependencies.Aliases, dependencies.Settings, dependencies.Storage, authenticator, dependencies.Logger)
		router.Get("/api/v1/images", imageHandler.list)
		router.Post("/api/v1/images", imageHandler.upload)
		router.Post("/api/v1/images/batch-delete", imageHandler.batchDelete)
		router.Patch("/api/v1/images/batch-visibility", imageHandler.batchVisibility)
		router.Get("/api/v1/images/{imageID}", imageHandler.detail)
		router.Delete("/api/v1/images/{imageID}", imageHandler.delete)
		router.Get("/api/v1/images/{imageID}/thumbnail", imageHandler.thumbnail)
		router.Patch("/api/v1/images/{imageID}/visibility", imageHandler.changeVisibility)
	}
	if dependencies.Importer != nil && dependencies.Auth != nil && dependencies.Settings != nil {
		importHandler := newImportHandler(dependencies.Importer, dependencies.Settings, authenticator)
		router.Post("/api/v1/imports", importHandler.create)
	}
	if dependencies.Settings != nil && dependencies.Auth != nil {
		settingsHandler := newSettingsHandler(dependencies.Settings, authenticator)
		router.Get("/api/v1/settings", settingsHandler.get)
		router.Patch("/api/v1/settings/default-visibility", settingsHandler.updateDefaultVisibility)
		router.Patch("/api/v1/settings/processing", settingsHandler.updateProcessing)
		systemHandler := newSystemHandler(dependencies.Settings, authenticator, dependencies.ProcessingConcurrency)
		router.Get("/api/v1/system", systemHandler.get)
	}
	if dependencies.Maintenance != nil && dependencies.Auth != nil {
		maintenanceHandler := newMaintenanceHandler(dependencies.Maintenance, authenticator)
		router.Get("/api/v1/overview", maintenanceHandler.overview)
		router.Post("/api/v1/maintenance/rebuild", maintenanceHandler.rebuild)
		router.Post("/api/v1/maintenance/inspect", maintenanceHandler.inspect)
	}
	var imageDelivery *deliveryHandler
	if dependencies.DeliveryIndex != nil && dependencies.Storage != nil {
		imageDelivery = newDeliveryHandler(dependencies.DeliveryIndex, dependencies.Storage, authenticator)
		router.Get("/image/{imageID}", imageDelivery.serve)
		router.Head("/image/{imageID}", imageDelivery.serve)
	}
	if dependencies.UI != nil {
		router.Handle("/assets/*", dependencies.UI.Assets())
		router.Handle("/brand/*", dependencies.UI.Assets())
		router.Get("/admin", dependencies.UI.ServeIndex)
		router.Get("/admin/*", dependencies.UI.ServeIndex)
	}
	if imageDelivery != nil {
		router.Get("/*", imageDelivery.serveAlias)
		router.Head("/*", imageDelivery.serveAlias)
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
