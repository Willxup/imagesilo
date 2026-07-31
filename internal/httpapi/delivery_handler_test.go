package httpapi

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestDeliveryUsesOnlyIndexAndFilesystem(t *testing.T) {
	content := []byte("jpeg-bytes-for-delivery")
	router, id := newDeliveryTestRouter(t, content)

	request := httptest.NewRequest(http.MethodGet, "/image/"+id, nil)
	request.Header.Set("Range", "bytes=0-3")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	result := response.Result()
	defer result.Body.Close()
	if result.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", result.StatusCode, http.StatusPartialContent)
	}
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != string(content[:4]) {
		t.Fatalf("body = %q, want %q", body, content[:4])
	}

	aliasRequest := httptest.NewRequest(http.MethodGet, "/legacy/sample.jpg?download=1", nil)
	aliasResponse := httptest.NewRecorder()
	router.ServeHTTP(aliasResponse, aliasRequest)
	if aliasResponse.Code != http.StatusOK || aliasResponse.Header().Get("Location") != "" {
		t.Fatalf("alias response = %d, Location = %q", aliasResponse.Code, aliasResponse.Header().Get("Location"))
	}
	if string(aliasResponse.Body.Bytes()) != string(content) {
		t.Fatalf("alias body = %q, want %q", aliasResponse.Body.Bytes(), content)
	}

	missRequest := httptest.NewRequest(http.MethodGet, "/legacy/missing.jpg", nil)
	missResponse := httptest.NewRecorder()
	router.ServeHTTP(missResponse, missRequest)
	if missResponse.Code != http.StatusNotFound {
		t.Fatalf("alias miss status = %d, want %d", missResponse.Code, http.StatusNotFound)
	}
}

func TestDeliveryRangeConditionalHeadAndConcurrentReads(t *testing.T) {
	content := []byte("jpeg-bytes-for-delivery")
	router, id := newDeliveryTestRouter(t, content)
	standardURL := "/image/" + id
	aliasURL := "/legacy/sample.jpg"

	prefix := deliveryRequest(router, http.MethodGet, standardURL, map[string]string{"Range": "bytes=2-5"})
	if prefix.Code != http.StatusPartialContent || prefix.Body.String() != string(content[2:6]) {
		t.Fatalf("prefix range = %d %q", prefix.Code, prefix.Body.String())
	}
	if prefix.Header().Get("Content-Range") != fmt.Sprintf("bytes 2-5/%d", len(content)) {
		t.Fatalf("prefix Content-Range = %q", prefix.Header().Get("Content-Range"))
	}

	suffix := deliveryRequest(router, http.MethodGet, aliasURL, map[string]string{"Range": "bytes=-4"})
	if suffix.Code != http.StatusPartialContent || suffix.Body.String() != string(content[len(content)-4:]) {
		t.Fatalf("suffix range = %d %q", suffix.Code, suffix.Body.String())
	}

	unsatisfiable := deliveryRequest(router, http.MethodGet, standardURL, map[string]string{"Range": "bytes=999-"})
	if unsatisfiable.Code != http.StatusRequestedRangeNotSatisfiable ||
		unsatisfiable.Header().Get("Content-Range") != fmt.Sprintf("bytes */%d", len(content)) {
		t.Fatalf("unsatisfiable range = %d, Content-Range = %q", unsatisfiable.Code, unsatisfiable.Header().Get("Content-Range"))
	}

	conditional := deliveryRequest(router, http.MethodGet, standardURL, map[string]string{"If-None-Match": `"etag"`})
	if conditional.Code != http.StatusNotModified || conditional.Body.Len() != 0 {
		t.Fatalf("conditional response = %d, body bytes = %d", conditional.Code, conditional.Body.Len())
	}
	modifiedSince := deliveryRequest(router, http.MethodGet, aliasURL, map[string]string{
		"If-Modified-Since": time.Unix(1_700_000_001, 0).UTC().Format(http.TimeFormat),
	})
	if modifiedSince.Code != http.StatusNotModified || modifiedSince.Body.Len() != 0 {
		t.Fatalf("If-Modified-Since response = %d, body bytes = %d", modifiedSince.Code, modifiedSince.Body.Len())
	}

	for _, path := range []string{standardURL, aliasURL} {
		head := deliveryRequest(router, http.MethodHead, path, nil)
		if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(len(content)) {
			t.Fatalf("HEAD %s = %d, body bytes = %d, Content-Length = %q", path, head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
		}
	}

	// Two tiny in-memory readers are enough to prove the immutable delivery index
	// and file-backed hot path are safe for overlap. This is not a load benchmark.
	var wait sync.WaitGroup
	failures := make(chan string, 2)
	for _, path := range []string{standardURL, aliasURL} {
		wait.Add(1)
		go func(path string) {
			defer wait.Done()
			response := deliveryRequest(router, http.MethodGet, path, map[string]string{"Range": "bytes=0-3"})
			if response.Code != http.StatusPartialContent || response.Body.String() != string(content[:4]) {
				failures <- fmt.Sprintf("GET %s = %d %q", path, response.Code, response.Body.String())
			}
		}(path)
	}
	wait.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

func TestDeliveryServesMigrationDirectoryAsPublicPathFallback(t *testing.T) {
	dataDirectory := t.TempDir()
	for _, directory := range []string{"images", "tmp", filepath.Join("migrations", "i", "2022", "04")} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, directory), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", directory, err)
		}
	}
	migrationContent := phaseTwoJPEG(t)
	migrationPath := filepath.Join(dataDirectory, "migrations", "i", "2022", "04", "sample.jpg")
	if err := os.WriteFile(migrationPath, migrationContent, 0o640); err != nil {
		t.Fatalf("WriteFile(migration) error = %v", err)
	}

	index := delivery.NewIndex()
	router := NewRouter(Dependencies{
		DeliveryIndex: index,
		Storage:       storage.NewFilesystem(dataDirectory),
		Logger:        slog.New(slog.DiscardHandler),
	})

	response := deliveryRequest(router, http.MethodGet, "/i/2022/04/sample.jpg", nil)
	if response.Code != http.StatusOK || response.Body.String() != string(migrationContent) {
		t.Fatalf("migration response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "image/jpeg" || response.Header().Get("Location") != "" {
		t.Fatalf("migration headers Content-Type = %q, Location = %q", response.Header().Get("Content-Type"), response.Header().Get("Location"))
	}
	ranged := deliveryRequest(router, http.MethodGet, "/i/2022/04/sample.jpg", map[string]string{"Range": "bytes=0-6"})
	if ranged.Code != http.StatusPartialContent || ranged.Body.String() != string(migrationContent[:7]) {
		t.Fatalf("migration range = %d %q", ranged.Code, ranged.Body.String())
	}
	head := deliveryRequest(router, http.MethodHead, "/i/2022/04/sample.jpg", nil)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(len(migrationContent)) {
		t.Fatalf("migration HEAD = %d, body = %d, Content-Length = %q", head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
	}
}

func TestDeliveryMigrationFallbackRejectsUnsafeAndNonImageFiles(t *testing.T) {
	dataDirectory := t.TempDir()
	migrationsDirectory := filepath.Join(dataDirectory, "migrations")
	if err := os.MkdirAll(migrationsDirectory, 0o750); err != nil {
		t.Fatalf("MkdirAll(migrations): %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDirectory, "notes.txt"), []byte("not public"), 0o640); err != nil {
		t.Fatalf("WriteFile(notes): %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDirectory, "disguised.jpg"), []byte("not an image"), 0o640); err != nil {
		t.Fatalf("WriteFile(disguised): %v", err)
	}
	outsidePath := filepath.Join(dataDirectory, "outside.jpg")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o640); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}
	requestPaths := []string{"/notes.txt", "/disguised.jpg", "/%2e%2e/outside.jpg"}
	if err := os.Symlink(outsidePath, filepath.Join(migrationsDirectory, "escape.jpg")); err == nil {
		requestPaths = append(requestPaths, "/escape.jpg")
	} else {
		t.Logf("Symlink unavailable: %v", err)
	}

	router := NewRouter(Dependencies{
		DeliveryIndex: delivery.NewIndex(),
		Storage:       storage.NewFilesystem(dataDirectory),
		Logger:        slog.New(slog.DiscardHandler),
	})
	for _, requestPath := range requestPaths {
		response := deliveryRequest(router, http.MethodGet, requestPath, nil)
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want %d", requestPath, response.Code, http.StatusNotFound)
		}
	}
}

func TestDeliveryAliasTakesPrecedenceOverMigrationFile(t *testing.T) {
	dataDirectory := t.TempDir()
	for _, directory := range []string{"images", "tmp", filepath.Join("migrations", "legacy")} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, directory), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", directory, err)
		}
	}
	const imageID = "019c1234-5678-7abc-8def-0123456789ab"
	aliasContent := []byte("managed-alias")
	if err := os.WriteFile(filepath.Join(dataDirectory, "images", imageID), aliasContent, 0o640); err != nil {
		t.Fatalf("WriteFile(alias target): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDirectory, "migrations", "legacy", "sample.jpg"), []byte("migration-file"), 0o640); err != nil {
		t.Fatalf("WriteFile(migration): %v", err)
	}
	index := delivery.NewIndex()
	index.Add(imageID, delivery.Target{
		StorageKey: imageID, MIMEType: "image/jpeg", ETag: `"alias"`, Size: int64(len(aliasContent)),
		LastModified: time.Unix(1_700_000_000, 0).UTC(), Visibility: "public", OriginalName: "sample.jpg",
	})
	index.AddAlias("/legacy/sample.jpg", imageID)
	router := NewRouter(Dependencies{DeliveryIndex: index, Storage: storage.NewFilesystem(dataDirectory), Logger: slog.New(slog.DiscardHandler)})
	response := deliveryRequest(router, http.MethodGet, "/legacy/sample.jpg", nil)
	if response.Code != http.StatusOK || response.Body.String() != string(aliasContent) {
		t.Fatalf("alias precedence response = %d %q", response.Code, response.Body.String())
	}
}

func TestDeliveryCapacityCoversStandardAliasAndMigrationPaths(t *testing.T) {
	dataDirectory := t.TempDir()
	for _, directory := range []string{"images", filepath.Join("migrations", "legacy")} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, directory), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", directory, err)
		}
	}
	const imageID = "019c1234-5678-7abc-8def-0123456789ab"
	content := []byte("image")
	if err := os.WriteFile(filepath.Join(dataDirectory, "images", imageID), content, 0o640); err != nil {
		t.Fatalf("WriteFile(image): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDirectory, "migrations", "legacy", "migration.jpg"), content, 0o640); err != nil {
		t.Fatalf("WriteFile(migration): %v", err)
	}
	index := delivery.NewIndex()
	index.Add(imageID, delivery.Target{
		StorageKey: imageID, MIMEType: "image/jpeg", ETag: `"etag"`, Size: int64(len(content)),
		LastModified: time.Now(), Visibility: "public", OriginalName: "sample.jpg",
	})
	index.AddAlias("/legacy/sample.jpg", imageID)
	gate := delivery.NewGate(1)
	release, ok := gate.TryAcquire()
	if !ok {
		t.Fatal("failed to occupy delivery gate")
	}
	defer release()
	router := NewRouter(Dependencies{
		DeliveryIndex: index, DeliveryGate: gate, Storage: storage.NewFilesystem(dataDirectory), Logger: slog.New(slog.DiscardHandler),
	})
	for _, requestPath := range []string{"/image/" + imageID, "/legacy/sample.jpg", "/legacy/migration.jpg"} {
		response := deliveryRequest(router, http.MethodGet, requestPath, nil)
		if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" {
			t.Fatalf("GET %s = %d, Retry-After = %q", requestPath, response.Code, response.Header().Get("Retry-After"))
		}
	}
}

func newDeliveryTestRouter(t *testing.T, content []byte) (http.Handler, string) {
	t.Helper()
	dataDirectory := t.TempDir()
	for _, path := range []string{"images", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", path, err)
		}
	}
	const id = "019c1234-5678-7abc-8def-0123456789ab"
	if err := os.WriteFile(filepath.Join(dataDirectory, "images", id), content, 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	index := delivery.NewIndex()
	index.Add(id, delivery.Target{
		StorageKey: id, MIMEType: "image/jpeg", ETag: `"etag"`, Size: int64(len(content)),
		LastModified: time.Unix(1_700_000_000, 0).UTC(), Visibility: "public", OriginalName: "sample.jpg",
	})
	index.AddAlias("/legacy/sample.jpg", id)
	closedDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	closedDB.Close()
	return NewRouter(Dependencies{
		DB: closedDB, Logger: slog.New(slog.DiscardHandler), DeliveryIndex: index, Storage: storage.NewFilesystem(dataDirectory),
	}), id
}

func deliveryRequest(router http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
