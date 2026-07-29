package httpapi

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestDeliveryUsesOnlyIndexAndFilesystem(t *testing.T) {
	dataDirectory := t.TempDir()
	for _, path := range []string{"images", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", path, err)
		}
	}
	const id = "019c1234-5678-7abc-8def-0123456789ab"
	content := []byte("jpeg-bytes-for-delivery")
	if err := os.WriteFile(filepath.Join(dataDirectory, "images", id), content, 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	index := delivery.NewIndex()
	index.Add(id, delivery.Target{
		StorageKey: id, MIMEType: "image/jpeg", ETag: `"etag"`, Size: int64(len(content)),
		LastModified: time.Unix(1_700_000_000, 0).UTC(), Visibility: "public", OriginalName: "sample.jpg",
	})

	closedDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	closedDB.Close()
	router := NewRouter(Dependencies{
		DB: closedDB, Logger: slog.New(slog.DiscardHandler), DeliveryIndex: index, Storage: storage.NewFilesystem(dataDirectory),
	})

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
}
