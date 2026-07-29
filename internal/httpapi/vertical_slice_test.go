package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestPhaseOneVerticalSlice(t *testing.T) {
	dataDirectory := t.TempDir()
	for _, path := range []string{"db", "images", filepath.Join("cache", "thumbnails"), "tmp"} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", path, err)
		}
	}
	databasePath := filepath.Join(dataDirectory, "db", "imagesilo.db")
	db, err := database.Open(databasePath)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	passwordHash, err := auth.HashPassword("phase-one-test-password")
	if err != nil {
		t.Fatalf("auth.HashPassword() error = %v", err)
	}
	now := time.Now().UTC()
	authRepository := auth.NewRepository(db)
	if err := authRepository.CreateAdmin(context.Background(), auth.Admin{
		ID: "administrator", Email: "admin@example.com", PasswordHash: passwordHash, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateAdmin() error = %v", err)
	}
	authService, err := auth.NewService(authRepository, auth.NewSessionIndex())
	if err != nil {
		t.Fatalf("auth.NewService() error = %v", err)
	}
	filesystem := storage.NewFilesystem(dataDirectory)
	index := delivery.NewIndex()
	imageService := images.NewService(images.NewRepository(db), filesystem, index)
	router := NewRouter(Dependencies{
		DB: db, Logger: slog.New(slog.DiscardHandler), Auth: authService, Images: imageService,
		DeliveryIndex: index, Storage: filesystem, CookieSecure: false,
	})

	loginBody := bytes.NewBufferString(`{"email":"admin@example.com","password":"phase-one-test-password"}`)
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName {
		t.Fatalf("login cookies = %+v, want one session cookie", cookies)
	}

	jpegBytes := verticalSliceJPEG(t)
	var uploadBody bytes.Buffer
	writer := multipart.NewWriter(&uploadBody)
	part, err := writer.CreateFormFile("file", "vertical.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(jpegBytes); err != nil {
		t.Fatalf("write multipart JPEG: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart.Close() error = %v", err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/images", &uploadBody)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRequest.AddCookie(cookies[0])
	uploadResponse := httptest.NewRecorder()
	router.ServeHTTP(uploadResponse, uploadRequest)
	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	var uploaded imageResponse
	if err := json.Unmarshal(uploadResponse.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/images", nil)
	listRequest.AddCookie(cookies[0])
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}

	reloadedIndex := delivery.NewIndex()
	if _, err := delivery.Load(context.Background(), db, filesystem, reloadedIndex); err != nil {
		t.Fatalf("delivery.Load() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}
	deliveryOnlyRouter := NewRouter(Dependencies{
		DB: db, Logger: slog.New(slog.DiscardHandler), DeliveryIndex: reloadedIndex, Storage: filesystem,
	})
	deliveryRequest := httptest.NewRequest(http.MethodGet, uploaded.StandardURL, nil)
	deliveryResponse := httptest.NewRecorder()
	deliveryOnlyRouter.ServeHTTP(deliveryResponse, deliveryRequest)
	if deliveryResponse.Code != http.StatusOK {
		t.Fatalf("delivery status after database close = %d", deliveryResponse.Code)
	}
	if !bytes.Equal(deliveryResponse.Body.Bytes(), jpegBytes) {
		t.Fatal("delivered bytes differ from uploaded JPEG")
	}
}

func verticalSliceJPEG(t *testing.T) []byte {
	t.Helper()
	value := stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 3))
	value.Set(0, 0, color.RGBA{G: 200, A: 255})
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, value, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buffer.Bytes()
}
