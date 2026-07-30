package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/setup"
)

func TestSetupStatusAndInitializeCreateFirstSession(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	authService, err := auth.NewService(auth.NewRepository(db), auth.NewSessionIndex())
	if err != nil {
		t.Fatalf("auth.NewService() error = %v", err)
	}
	router := NewRouter(Dependencies{
		DB: db, Logger: slog.New(slog.DiscardHandler), Auth: authService, Setup: setup.NewService(db),
	})

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	statusResponse := httptest.NewRecorder()
	router.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK || !strings.Contains(statusResponse.Body.String(), `"initialized":false`) {
		t.Fatalf("initial setup status = %d %s", statusResponse.Code, statusResponse.Body.String())
	}

	body := `{"displayName":"Will","email":"ADMIN@example.com","password":"a secure setup password","defaultVisibility":"private","compressionEnabled":false,"jpegQuality":85,"webpQuality":82,"pngCompressionLevel":6,"conversionEnabled":false,"conversionWebpQuality":82,"conversionWebpLossless":false}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("setup response = %d %s", response.Code, response.Body.String())
	}
	var session sessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode setup session: %v", err)
	}
	if session.DisplayName != "Will" || session.Email != "admin@example.com" || session.CSRFToken == "" {
		t.Fatalf("setup session = %+v", session)
	}
	if len(response.Result().Cookies()) < 2 {
		t.Fatalf("setup cookies = %+v", response.Result().Cookies())
	}

	second := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(body))
	secondRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(second, secondRequest)
	if second.Code != http.StatusConflict {
		t.Fatalf("second setup response = %d %s", second.Code, second.Body.String())
	}
}
