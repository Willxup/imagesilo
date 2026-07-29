package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestPhaseFourAliasAPIConflictAndDirectDelivery(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	jpegBytes := phaseTwoJPEG(t)
	firstImage := fixture.uploadBytes(cookies, csrfToken, "public", "", "first.jpg", jpegBytes)
	secondImage := fixture.uploadBytes(cookies, csrfToken, "public", "", "second.jpg", jpegBytes)

	createdResponse := fixture.request(http.MethodPost, "/api/v1/aliases", map[string]any{
		"path": "/legacy/旧图.jpg", "imageId": firstImage.ID, "source": "phase-four-test",
	}, cookies, csrfToken, "")
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create alias status = %d, body = %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created aliasResponse
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode alias response: %v", err)
	}
	if created.Path != "/legacy/%E6%97%A7%E5%9B%BE.jpg" || created.ImageID != firstImage.ID {
		t.Fatalf("created alias = %+v", created)
	}

	conflict := fixture.request(http.MethodPost, "/api/v1/aliases", map[string]any{
		"path": created.Path, "imageId": secondImage.ID, "source": "must-not-overwrite",
	}, cookies, csrfToken, "")
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, body = %s", conflict.Code, conflict.Body.String())
	}
	if imageID, ok := fixture.deliveryIndex.ResolveAlias(created.Path); !ok || imageID != firstImage.ID {
		t.Fatalf("conflict changed alias target to %q, found = %t", imageID, ok)
	}

	list := fixture.request(http.MethodGet, "/api/v1/aliases?limit=10", nil, cookies, "", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list aliases status = %d, body = %s", list.Code, list.Body.String())
	}
	var listed aliasListResponse
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || len(listed.Items) != 1 || listed.Items[0].ID != created.ID {
		t.Fatalf("list aliases = %+v, error = %v", listed, err)
	}
	resolved := fixture.request(http.MethodGet, "/api/v1/aliases/resolve?path="+url.QueryEscape("/legacy/旧图.jpg"), nil, cookies, "", "")
	if resolved.Code != http.StatusOK {
		t.Fatalf("resolve alias status = %d, body = %s", resolved.Code, resolved.Body.String())
	}

	delivered := fixture.request(http.MethodGet, created.Path, nil, nil, "", "")
	if delivered.Code != http.StatusOK || delivered.Header().Get("Location") != "" || !bytes.Equal(delivered.Body.Bytes(), jpegBytes) {
		t.Fatalf("direct alias delivery status = %d, Location = %q, bytes equal = %t", delivered.Code, delivered.Header().Get("Location"), bytes.Equal(delivered.Body.Bytes(), jpegBytes))
	}
	standard := fixture.request(http.MethodGet, firstImage.StandardURL, nil, nil, "", "")
	if standard.Code != http.StatusOK || !bytes.Equal(standard.Body.Bytes(), delivered.Body.Bytes()) {
		t.Fatal("standard and alias URL did not return identical bytes")
	}

	reserved := fixture.request(http.MethodPost, "/api/v1/aliases", map[string]any{
		"path": "/admin/settings", "imageId": firstImage.ID, "source": "invalid",
	}, cookies, csrfToken, "")
	if reserved.Code != http.StatusBadRequest {
		t.Fatalf("reserved alias status = %d, body = %s", reserved.Code, reserved.Body.String())
	}

	deleted := fixture.request(http.MethodDelete, "/api/v1/aliases/"+created.ID, nil, cookies, csrfToken, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete alias status = %d, body = %s", deleted.Code, deleted.Body.String())
	}
	if response := fixture.request(http.MethodGet, created.Path, nil, nil, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("deleted alias delivery status = %d", response.Code)
	}
}

func TestPhaseFourAliasInheritsPrivateVisibility(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	privateImage := fixture.uploadBytes(cookies, csrfToken, "private", "", "private.jpg", phaseTwoJPEG(t))
	created := fixture.request(http.MethodPost, "/api/v1/aliases", map[string]any{
		"path": "/legacy/private.jpg", "imageId": privateImage.ID, "source": "phase-four-test",
	}, cookies, csrfToken, "")
	if created.Code != http.StatusCreated {
		t.Fatalf("create private alias status = %d, body = %s", created.Code, created.Body.String())
	}

	unauthorized := fixture.request(http.MethodGet, "/legacy/private.jpg", nil, nil, "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("private alias unauthenticated status = %d", unauthorized.Code)
	}
	aliasWriter := fixture.createToken(cookies, csrfToken, "alias writer", []apitoken.Scope{apitoken.ScopeAliasesWrite})
	insufficient := fixture.request(http.MethodGet, "/legacy/private.jpg", nil, nil, "", aliasWriter.Token)
	if insufficient.Code != http.StatusForbidden {
		t.Fatalf("private alias wrong scope status = %d", insufficient.Code)
	}
	reader := fixture.createToken(cookies, csrfToken, "private reader", []apitoken.Scope{apitoken.ScopeImagesReadPrivate})
	allowed := fixture.request(http.MethodGet, "/legacy/private.jpg", nil, nil, "", reader.Token)
	if allowed.Code != http.StatusOK || allowed.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("private alias reader status = %d, Cache-Control = %q", allowed.Code, allowed.Header().Get("Cache-Control"))
	}
}

func TestPhaseFourAliasWriteScopeControlsManagementAPI(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	image := fixture.uploadBytes(cookies, csrfToken, "public", "", "scope.jpg", phaseTwoJPEG(t))
	wrongScope := fixture.createToken(cookies, csrfToken, "uploader", []apitoken.Scope{apitoken.ScopeImagesUpload})
	aliasWriter := fixture.createToken(cookies, csrfToken, "alias writer", []apitoken.Scope{apitoken.ScopeAliasesWrite})
	request := map[string]any{"path": "/legacy/scope.jpg", "imageId": image.ID, "source": "scope-test"}
	forbidden := fixture.request(http.MethodPost, "/api/v1/aliases", request, nil, "", wrongScope.Token)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("wrong-scope create status = %d, body = %s", forbidden.Code, forbidden.Body.String())
	}
	createdResponse := fixture.request(http.MethodPost, "/api/v1/aliases", request, nil, "", aliasWriter.Token)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("alias-writer create status = %d, body = %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created aliasResponse
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode alias response: %v", err)
	}
	if list := fixture.request(http.MethodGet, "/api/v1/aliases", nil, nil, "", aliasWriter.Token); list.Code != http.StatusOK {
		t.Fatalf("alias-writer list status = %d, body = %s", list.Code, list.Body.String())
	}
	if deleted := fixture.request(http.MethodDelete, "/api/v1/aliases/"+created.ID, nil, nil, "", aliasWriter.Token); deleted.Code != http.StatusNoContent {
		t.Fatalf("alias-writer delete status = %d, body = %s", deleted.Code, deleted.Body.String())
	}
}

func TestPhaseFourAliasReloadSurvivesDatabaseClosure(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	jpegBytes := phaseTwoJPEG(t)
	image := fixture.uploadBytes(cookies, csrfToken, "public", "", "restart.jpg", jpegBytes)
	created := fixture.request(http.MethodPost, "/api/v1/aliases", map[string]any{
		"path": "/legacy/restart.jpg", "imageId": image.ID, "source": "restart-test",
	}, cookies, csrfToken, "")
	if created.Code != http.StatusCreated {
		t.Fatalf("create alias status = %d, body = %s", created.Code, created.Body.String())
	}

	reloaded := delivery.NewIndex()
	filesystem := storage.NewFilesystem(fixture.dataDirectory)
	if _, err := delivery.Load(context.Background(), fixture.db, filesystem, reloaded); err != nil {
		t.Fatalf("delivery.Load() error = %v", err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}
	router := NewRouter(Dependencies{
		DB: fixture.db, Logger: slog.New(slog.DiscardHandler), DeliveryIndex: reloaded, Storage: filesystem,
	})
	for _, path := range []string{image.StandardURL, "/legacy/restart.jpg"} {
		response := httptestRequest(router, path)
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), jpegBytes) {
			t.Fatalf("delivery %s after database close = %d, bytes equal = %t", path, response.Code, bytes.Equal(response.Body.Bytes(), jpegBytes))
		}
	}
	if response := httptestRequest(router, "/legacy/not-found.jpg"); response.Code != http.StatusNotFound {
		t.Fatalf("alias miss after database close status = %d", response.Code)
	}
}

func httptestRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
