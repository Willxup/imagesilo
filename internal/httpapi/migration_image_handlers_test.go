package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Willxup/imagesilo/internal/migrationimage"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestMigrationImageManagementListsDeliversAndDeletes(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	relativePath := filepath.Join("i", "2026", "08", "sample.jpg")
	filePath := filepath.Join(fixture.dataDirectory, "migrations", relativePath)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
		t.Fatalf("MkdirAll(migration): %v", err)
	}
	imageBytes := phaseTwoJPEG(t)
	if err := os.WriteFile(filePath, imageBytes, 0o640); err != nil {
		t.Fatalf("WriteFile(migration): %v", err)
	}

	if unauthorized := fixture.request(http.MethodGet, "/api/v1/migration-images", nil, nil, "", ""); unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized list status = %d", unauthorized.Code)
	}
	list := fixture.request(http.MethodGet, "/api/v1/migration-images?limit=24&format=jpeg&q=sample", nil, cookies, "", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", list.Code, list.Body.String())
	}
	var listed migrationImageListResponse
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || len(listed.Items) != 1 || listed.Items[0].Path != "/i/2026/08/sample.jpg" || !listed.MutationsEnabled {
		t.Fatalf("listed migration images = %+v, error = %v", listed, err)
	}
	delivered := fixture.request(http.MethodGet, listed.Items[0].StandardURL, nil, nil, "", "")
	if delivered.Code != http.StatusOK {
		t.Fatalf("migration delivery status = %d", delivered.Code)
	}

	missingCSRF := fixture.request(http.MethodPost, "/api/v1/migration-images/batch-delete", map[string]any{
		"paths": []string{listed.Items[0].Path},
	}, cookies, "", "")
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("delete without CSRF status = %d", missingCSRF.Code)
	}
	deleted := fixture.request(http.MethodPost, "/api/v1/migration-images/batch-delete", map[string]any{
		"paths": []string{listed.Items[0].Path},
	}, cookies, csrfToken, "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleted.Code, deleted.Body.String())
	}
	var result migrationImageBatchResult
	if err := json.Unmarshal(deleted.Body.Bytes(), &result); err != nil || len(result.Items) != 1 || result.Items[0].Status != "deleted" || result.Items[0].RemovedDirectories != 3 {
		t.Fatalf("delete result = %+v, error = %v", result, err)
	}
	if response := fixture.request(http.MethodGet, listed.Items[0].StandardURL, nil, nil, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("deleted migration delivery status = %d", response.Code)
	}
	if _, err := os.Stat(filepath.Join(fixture.dataDirectory, "migrations")); err != nil {
		t.Fatalf("migration root was removed: %v", err)
	}
}

func TestMigrationImageDeletionRequiresExplicitCapability(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	filesystem := storage.NewFilesystem(fixture.dataDirectory)
	readOnlyRouter := NewRouter(Dependencies{
		DB: fixture.db, Logger: slog.New(slog.DiscardHandler), Auth: fixture.authService,
		MigrationImages: migrationimage.NewService(filesystem, false),
	})
	response := requestWithRouter(t, readOnlyRouter, http.MethodPost, "/api/v1/migration-images/batch-delete", map[string]any{
		"paths": []string{"/i/2026/sample.jpg"},
	}, cookies, csrfToken)
	if response.Code != http.StatusForbidden {
		t.Fatalf("read-only delete status = %d, body = %s", response.Code, response.Body.String())
	}
	var apiError errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &apiError); err != nil || apiError.Code != "migration_mutations_disabled" {
		t.Fatalf("read-only delete error = %+v, %v", apiError, err)
	}
}

func TestMigrationImageRefreshRequiresCSRFAndRebuildsSnapshot(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	writeImage := func(relativePath string) {
		filePath := filepath.Join(fixture.dataDirectory, "migrations", filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
			t.Fatalf("MkdirAll(migration): %v", err)
		}
		if err := os.WriteFile(filePath, phaseTwoJPEG(t), 0o640); err != nil {
			t.Fatalf("WriteFile(migration): %v", err)
		}
	}
	writeImage("i/first.jpg")

	initial := fixture.request(http.MethodGet, "/api/v1/migration-images", nil, cookies, "", "")
	var initialList migrationImageListResponse
	if initial.Code != http.StatusOK || json.Unmarshal(initial.Body.Bytes(), &initialList) != nil || len(initialList.Items) != 1 {
		t.Fatalf("initial list status = %d, body = %s", initial.Code, initial.Body.String())
	}
	writeImage("images/second.jpg")
	cached := fixture.request(http.MethodGet, "/api/v1/migration-images", nil, cookies, "", "")
	var cachedList migrationImageListResponse
	if cached.Code != http.StatusOK || json.Unmarshal(cached.Body.Bytes(), &cachedList) != nil || len(cachedList.Items) != 1 {
		t.Fatalf("cached list status = %d, body = %s", cached.Code, cached.Body.String())
	}

	if unauthorized := fixture.request(http.MethodPost, "/api/v1/migration-images/refresh", nil, nil, "", ""); unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized refresh status = %d", unauthorized.Code)
	}
	if missingCSRF := fixture.request(http.MethodPost, "/api/v1/migration-images/refresh", nil, cookies, "", ""); missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("refresh without CSRF status = %d", missingCSRF.Code)
	}
	if refreshed := fixture.request(http.MethodPost, "/api/v1/migration-images/refresh", nil, cookies, csrfToken, ""); refreshed.Code != http.StatusNoContent {
		t.Fatalf("refresh status = %d, body = %s", refreshed.Code, refreshed.Body.String())
	}
	list := fixture.request(http.MethodGet, "/api/v1/migration-images", nil, cookies, "", "")
	var refreshedList migrationImageListResponse
	if list.Code != http.StatusOK || json.Unmarshal(list.Body.Bytes(), &refreshedList) != nil || len(refreshedList.Items) != 2 {
		t.Fatalf("refreshed list status = %d, body = %s", list.Code, list.Body.String())
	}
}

func requestWithRouter(t *testing.T, router http.Handler, method, path string, value any, cookies []*http.Cookie, csrfToken string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(): %v", err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	addAuthentication(request, cookies, csrfToken, "")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
