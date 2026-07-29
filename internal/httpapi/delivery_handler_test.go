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
