package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/config"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/platform/database"
)

func TestUntilNextMaintenanceUsesConfiguredUTCHour(t *testing.T) {
	now := time.Date(2026, time.July, 29, 2, 30, 0, 0, time.UTC)
	if delay := untilNextMaintenance(now, 3); delay != 30*time.Minute {
		t.Fatalf("delay before maintenance hour = %v", delay)
	}
	if delay := untilNextMaintenance(now.Add(time.Hour), 3); delay != 23*time.Hour+30*time.Minute {
		t.Fatalf("delay after maintenance hour = %v", delay)
	}
}

func TestBuildKeepsServingWhenOneDatabaseImageFileIsMissing(t *testing.T) {
	cfg := config.Config{
		ListenAddress: "127.0.0.1:0", DataDirectory: filepath.Join(t.TempDir(), "data"),
		ProcessingConcurrency: 1, ShutdownTimeout: 5 * time.Second, CookieSecure: false,
	}
	if err := cfg.PrepareDataDirectories(); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(cfg.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	passwordHash, err := auth.HashPassword("missing-file-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewRepository(db).CreateAdmin(context.Background(), auth.Admin{
		ID: "administrator", Email: "admin@example.com", PasswordHash: passwordHash, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	var digest [32]byte
	digest[0] = 1
	missingID := "019c1234-5678-7abc-8def-0123456789ab"
	if err := images.NewRepository(db).Create(context.Background(), images.Image{
		ID: missingID, OriginalName: "missing.jpg", StorageKey: missingID, Extension: ".jpg", MIMEType: "image/jpeg",
		Width: 1, Height: 1, SourceSize: 1, StoredSize: 1, SourceSHA256: digest, StoredSHA256: digest,
		ProcessingSummary: "{}", Visibility: images.VisibilityPublic, UploadedVia: "admin", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	application, err := Build(context.Background(), cfg, db, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Build() failed because one formal file was missing: %v", err)
	}
	defer application.Close()

	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"missing-file-test-password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	application.Handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	overviewRequest := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	for _, cookie := range loginResponse.Result().Cookies() {
		overviewRequest.AddCookie(cookie)
	}
	overviewResponse := httptest.NewRecorder()
	application.Handler.ServeHTTP(overviewResponse, overviewRequest)
	var overview struct {
		MissingImageCount int      `json:"missingImageCount"`
		MissingImageIDs   []string `json:"missingImageIds"`
	}
	if err := json.Unmarshal(overviewResponse.Body.Bytes(), &overview); overviewResponse.Code != http.StatusOK || err != nil ||
		overview.MissingImageCount != 1 || len(overview.MissingImageIDs) != 1 || overview.MissingImageIDs[0] != missingID {
		t.Fatalf("overview status = %d, value = %+v, error = %v", overviewResponse.Code, overview, err)
	}
	deliveryResponse := httptest.NewRecorder()
	application.Handler.ServeHTTP(deliveryResponse, httptest.NewRequest(http.MethodGet, "/image/"+missingID, nil))
	if deliveryResponse.Code != http.StatusNotFound {
		t.Fatalf("missing image delivery status = %d", deliveryResponse.Code)
	}
}
