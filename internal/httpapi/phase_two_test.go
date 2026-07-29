package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
	"github.com/Willxup/imagesilo/internal/settings"
)

const phaseTwoPassword = "phase-two-secure-password"

type phaseTwoFixture struct {
	t              *testing.T
	db             *sql.DB
	router         http.Handler
	authService    *auth.Service
	tokenService   *apitoken.Service
	dataDirectory  string
	sessionCookies []*http.Cookie
	csrfToken      string
	logs           *bytes.Buffer
}

func TestPhaseTwoSessionRotationCSRFRateLimitAndHeaders(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	firstCookies, firstCSRF, firstResponse := fixture.login(nil, phaseTwoPassword)
	if firstResponse.Header().Get("Content-Security-Policy") == "" || firstResponse.Header().Get("Permissions-Policy") == "" {
		t.Fatal("security headers are missing from login response")
	}
	firstSession := cookieByName(t, firstCookies, sessionCookieName)
	firstCSRFCookie := cookieByName(t, firstCookies, csrfCookieName)
	if !firstSession.HttpOnly || firstSession.SameSite != http.SameSiteLaxMode || firstCSRFCookie.HttpOnly {
		t.Fatalf("unexpected session cookie security attributes: session=%+v csrf=%+v", firstSession, firstCSRFCookie)
	}
	if firstCSRF == "" || firstCSRFCookie.Value != firstCSRF {
		t.Fatal("login did not bind the CSRF cookie and response token")
	}

	secondCookies, secondCSRF, _ := fixture.login(firstCookies, phaseTwoPassword)
	secondSession := cookieByName(t, secondCookies, sessionCookieName)
	if secondSession.Value == firstSession.Value || secondCSRF == firstCSRF {
		t.Fatal("login did not rotate session and CSRF tokens")
	}
	if _, err := fixture.authService.Authenticate(firstSession.Value, time.Now()); !errors.Is(err, auth.ErrInvalidSession) {
		t.Fatalf("rotated session error = %v, want ErrInvalidSession", err)
	}

	missingCSRF := fixture.request(http.MethodPost, "/api/v1/auth/logout", nil, secondCookies, "", "")
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF status = %d, body = %s", missingCSRF.Code, missingCSRF.Body.String())
	}
	current := fixture.request(http.MethodGet, "/api/v1/auth/session", nil, secondCookies, "", "")
	if current.Code != http.StatusOK {
		t.Fatalf("current session status = %d, body = %s", current.Code, current.Body.String())
	}
	logout := fixture.request(http.MethodPost, "/api/v1/auth/logout", nil, secondCookies, secondCSRF, "")
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", logout.Code, logout.Body.String())
	}
	if _, err := fixture.authService.Authenticate(secondSession.Value, time.Now()); !errors.Is(err, auth.ErrInvalidSession) {
		t.Fatalf("logged-out session error = %v, want ErrInvalidSession", err)
	}

	for attempt := 1; attempt <= 6; attempt++ {
		response := fixture.loginWithEmail("missing@example.com", "wrong password", nil)
		if attempt <= 5 && response.Code != http.StatusUnauthorized {
			t.Fatalf("invalid login attempt %d status = %d", attempt, response.Code)
		}
		if attempt == 6 {
			if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
				t.Fatalf("rate-limited login status = %d, Retry-After = %q", response.Code, response.Header().Get("Retry-After"))
			}
		}
	}
}

func TestPhaseTwoTokenVisibilityAndPrivateDeliveryUseMemoryIndexes(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)

	readToken := fixture.createToken(cookies, csrfToken, "private reader", []apitoken.Scope{apitoken.ScopeImagesReadPrivate})
	liveReadToken := fixture.createToken(cookies, csrfToken, "restart reader", []apitoken.Scope{apitoken.ScopeImagesReadPrivate})
	uploadToken := fixture.createToken(cookies, csrfToken, "uploader", []apitoken.Scope{apitoken.ScopeImagesUpload})

	settingsResponse := fixture.request(http.MethodPatch, "/api/v1/settings/default-visibility", map[string]any{
		"defaultVisibility": "private",
	}, cookies, csrfToken, "")
	if settingsResponse.Code != http.StatusOK {
		t.Fatalf("update default visibility status = %d, body = %s", settingsResponse.Code, settingsResponse.Body.String())
	}

	privateImage := fixture.upload(cookies, csrfToken, "", "")
	if privateImage.Visibility != images.VisibilityPrivate {
		t.Fatalf("default upload visibility = %s, want private", privateImage.Visibility)
	}
	unauthorized := fixture.request(http.MethodGet, privateImage.StandardURL, nil, nil, "", "")
	if unauthorized.Code != http.StatusUnauthorized || unauthorized.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("private unauthenticated response = %d, Cache-Control = %q", unauthorized.Code, unauthorized.Header().Get("Cache-Control"))
	}
	wrongScope := fixture.request(http.MethodGet, privateImage.StandardURL, nil, nil, "", uploadToken.Token)
	if wrongScope.Code != http.StatusForbidden {
		t.Fatalf("private wrong-scope status = %d, body = %s", wrongScope.Code, wrongScope.Body.String())
	}
	invalid := fixture.request(http.MethodGet, privateImage.StandardURL, nil, nil, "", "ist_invalid")
	if invalid.Code != http.StatusUnauthorized {
		t.Fatalf("private invalid-token status = %d, body = %s", invalid.Code, invalid.Body.String())
	}
	privateByToken := fixture.request(http.MethodGet, privateImage.StandardURL, nil, nil, "", readToken.Token)
	if privateByToken.Code != http.StatusOK || privateByToken.Header().Get("Vary") != "Authorization, Cookie" {
		t.Fatalf("private token response = %d, Vary = %q", privateByToken.Code, privateByToken.Header().Get("Vary"))
	}
	queryToken := fixture.request(http.MethodGet, privateImage.StandardURL+"?token="+readToken.Token, nil, nil, "", "")
	if queryToken.Code != http.StatusBadRequest {
		t.Fatalf("query-string token status = %d, body = %s", queryToken.Code, queryToken.Body.String())
	}

	publicImage := fixture.upload(nil, "", "public", uploadToken.Token)
	if publicImage.Visibility != images.VisibilityPublic {
		t.Fatalf("explicit upload visibility = %s, want public", publicImage.Visibility)
	}
	publicResponse := fixture.request(http.MethodGet, publicImage.StandardURL, nil, nil, "", "")
	if publicResponse.Code != http.StatusOK || publicResponse.Header().Get("Cache-Control") != "public, no-cache" {
		t.Fatalf("public response = %d, Cache-Control = %q", publicResponse.Code, publicResponse.Header().Get("Cache-Control"))
	}
	visibilityResponse := fixture.request(http.MethodPatch, "/api/v1/images/"+publicImage.ID+"/visibility", map[string]any{
		"visibility": "private",
	}, cookies, csrfToken, "")
	if visibilityResponse.Code != http.StatusNoContent {
		t.Fatalf("change visibility status = %d, body = %s", visibilityResponse.Code, visibilityResponse.Body.String())
	}
	if response := fixture.request(http.MethodGet, publicImage.StandardURL, nil, nil, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("image remained public after visibility change: status = %d", response.Code)
	}

	revoke := fixture.request(http.MethodDelete, "/api/v1/api-tokens/"+readToken.ID, nil, cookies, csrfToken, "")
	if revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke token status = %d, body = %s", revoke.Code, revoke.Body.String())
	}
	if response := fixture.request(http.MethodGet, privateImage.StandardURL, nil, nil, "", readToken.Token); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token private-read status = %d", response.Code)
	}
	list := fixture.request(http.MethodGet, "/api/v1/api-tokens", nil, cookies, "", "")
	if list.Code != http.StatusOK || bytes.Contains(list.Body.Bytes(), []byte(readToken.Token)) || bytes.Contains(list.Body.Bytes(), []byte(liveReadToken.Token)) {
		t.Fatalf("token list leaked plaintext or failed: status = %d, body = %s", list.Code, list.Body.String())
	}
	for _, secret := range []string{
		phaseTwoPassword,
		cookieByName(t, cookies, sessionCookieName).Value,
		readToken.Token,
		liveReadToken.Token,
		uploadToken.Token,
	} {
		if bytes.Contains(fixture.logs.Bytes(), []byte(secret)) {
			t.Fatalf("application log contains plaintext authentication secret %q", secret)
		}
	}
	var storedPasswordHash string
	if err := fixture.db.QueryRow("SELECT password_hash FROM admin WHERE singleton = 1").Scan(&storedPasswordHash); err != nil {
		t.Fatalf("query stored administrator password hash: %v", err)
	}
	if storedPasswordHash == phaseTwoPassword || !bytes.HasPrefix([]byte(storedPasswordHash), []byte("$argon2id$")) {
		t.Fatal("database does not contain an Argon2id-only administrator password")
	}

	if err := fixture.db.Close(); err != nil {
		t.Fatalf("close database before hot-path proof: %v", err)
	}
	privateBySession := fixture.request(http.MethodGet, privateImage.StandardURL, nil, cookies, "", "")
	if privateBySession.Code != http.StatusOK {
		t.Fatalf("private session read after database close = %d, body = %s", privateBySession.Code, privateBySession.Body.String())
	}
	privateByLiveToken := fixture.request(http.MethodGet, privateImage.StandardURL, nil, nil, "", liveReadToken.Token)
	if privateByLiveToken.Code != http.StatusOK {
		t.Fatalf("private token read after database close = %d, body = %s", privateByLiveToken.Code, privateByLiveToken.Body.String())
	}
}

func newPhaseTwoFixture(t *testing.T) *phaseTwoFixture {
	return newHTTPFixture(t, processor.NewEngine(), processor.NewGate(1), 1)
}

func newHTTPFixture(t *testing.T, engine processor.Engine, gate *processor.Gate, processingConcurrency int) *phaseTwoFixture {
	t.Helper()
	dataDirectory := t.TempDir()
	for _, path := range []string{"db", "images", filepath.Join("cache", "thumbnails"), "tmp"} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", path, err)
		}
	}
	db, err := database.Open(filepath.Join(dataDirectory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	passwordHash, err := auth.HashPassword(phaseTwoPassword)
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
	tokenService := apitoken.NewService(apitoken.NewRepository(db), apitoken.NewIndex())
	filesystem := storage.NewFilesystem(dataDirectory)
	deliveryIndex := delivery.NewIndex()
	imageService := images.NewServiceWithProcessor(images.NewRepository(db), filesystem, deliveryIndex, engine, gate)
	settingsService := settings.NewService(settings.NewRepository(db))
	var logs bytes.Buffer
	router := NewRouter(Dependencies{
		DB: db, Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Auth: authService, APITokens: tokenService,
		Images: imageService, Settings: settingsService, DeliveryIndex: deliveryIndex, Storage: filesystem,
		CookieSecure: false, ProcessingConcurrency: processingConcurrency,
	})
	return &phaseTwoFixture{
		t: t, db: db, router: router, authService: authService, tokenService: tokenService, dataDirectory: dataDirectory, logs: &logs,
	}
}

func (f *phaseTwoFixture) login(cookies []*http.Cookie, password string) ([]*http.Cookie, string, *httptest.ResponseRecorder) {
	f.t.Helper()
	response := f.loginWithEmail("admin@example.com", password, cookies)
	if response.Code != http.StatusOK {
		f.t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	var session sessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		f.t.Fatalf("decode login response: %v", err)
	}
	return response.Result().Cookies(), session.CSRFToken, response
}

func (f *phaseTwoFixture) loginWithEmail(email, password string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.request(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": email, "password": password,
	}, cookies, "", "")
}

func (f *phaseTwoFixture) createToken(cookies []*http.Cookie, csrfToken, name string, scopes []apitoken.Scope) apiTokenResponse {
	f.t.Helper()
	response := f.request(http.MethodPost, "/api/v1/api-tokens", map[string]any{
		"name": name, "scopes": scopes,
	}, cookies, csrfToken, "")
	if response.Code != http.StatusCreated {
		f.t.Fatalf("create token status = %d, body = %s", response.Code, response.Body.String())
	}
	var token apiTokenResponse
	if err := json.Unmarshal(response.Body.Bytes(), &token); err != nil {
		f.t.Fatalf("decode API token response: %v", err)
	}
	if token.Token == "" || token.TokenPrefix == token.Token {
		f.t.Fatal("API token plaintext was not returned exactly at creation")
	}
	return token
}

func (f *phaseTwoFixture) upload(cookies []*http.Cookie, csrfToken, visibility, bearer string) imageResponse {
	f.t.Helper()
	return f.uploadBytes(cookies, csrfToken, visibility, bearer, "phase-two.jpg", phaseTwoJPEG(f.t))
}

func (f *phaseTwoFixture) uploadBytes(cookies []*http.Cookie, csrfToken, visibility, bearer, filename string, data []byte) imageResponse {
	f.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		f.t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(data); err != nil {
		f.t.Fatalf("write JPEG: %v", err)
	}
	if visibility != "" {
		if err := writer.WriteField("visibility", visibility); err != nil {
			f.t.Fatalf("WriteField(visibility) error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		f.t.Fatalf("multipart.Close() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/images", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	addAuthentication(request, cookies, csrfToken, bearer)
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		f.t.Fatalf("upload status = %d, body = %s", response.Code, response.Body.String())
	}
	var image imageResponse
	if err := json.Unmarshal(response.Body.Bytes(), &image); err != nil {
		f.t.Fatalf("decode image response: %v", err)
	}
	return image
}

func (f *phaseTwoFixture) request(method, path string, value any, cookies []*http.Cookie, csrfToken, bearer string) *httptest.ResponseRecorder {
	f.t.Helper()
	var body bytes.Buffer
	if value != nil {
		if err := json.NewEncoder(&body).Encode(value); err != nil {
			f.t.Fatalf("encode request body: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, &body)
	if value != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	addAuthentication(request, cookies, csrfToken, bearer)
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	return response
}

func addAuthentication(request *http.Request, cookies []*http.Cookie, csrfToken, bearer string) {
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	if csrfToken != "" {
		request.Header.Set("X-CSRF-Token", csrfToken)
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
}

func cookieByName(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %s was not set: %+v", name, cookies)
	return nil
}

func phaseTwoJPEG(t *testing.T) []byte {
	t.Helper()
	value := stdimage.NewRGBA(stdimage.Rect(0, 0, 5, 4))
	value.Set(0, 0, color.RGBA{B: 220, A: 255})
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, value, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buffer.Bytes()
}
