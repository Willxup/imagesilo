package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Willxup/imagesilo/internal/apitoken"
)

func TestPhaseSixImportRequiresBothScopesAndIsAtomicOnConflict(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	uploadOnly := fixture.createToken(cookies, csrfToken, "upload only", []apitoken.Scope{apitoken.ScopeImagesUpload})
	aliasOnly := fixture.createToken(cookies, csrfToken, "alias only", []apitoken.Scope{apitoken.ScopeAliasesWrite})
	both := fixture.createToken(cookies, csrfToken, "importer", []apitoken.Scope{apitoken.ScopeImagesUpload, apitoken.ScopeAliasesWrite})
	data := phaseTwoJPEG(t)
	for name, token := range map[string]string{"upload": uploadOnly.Token, "alias": aliasOnly.Token} {
		response := fixture.importImage(nil, "", token, "/legacy/"+name+".jpg", "public", data)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s-only token import status = %d", name, response.Code)
		}
	}

	response := fixture.importImage(nil, "", both.Token, "/legacy/imported.jpg", "public", data)
	var imported importResponse
	if err := json.Unmarshal(response.Body.Bytes(), &imported); response.Code != http.StatusCreated || err != nil {
		t.Fatalf("import status = %d, value = %+v, error = %v, body = %s", response.Code, imported, err, response.Body.String())
	}
	if imported.ImageID == "" || imported.Alias.Path != "/legacy/imported.jpg" || imported.Alias.ImageID != imported.ImageID {
		t.Fatalf("import response = %+v", imported)
	}
	if delivered := fixture.request(http.MethodGet, imported.Alias.Path, nil, nil, "", ""); delivered.Code != http.StatusOK || !bytes.Equal(delivered.Body.Bytes(), data) {
		t.Fatalf("imported alias delivery status = %d", delivered.Code)
	}
	var imageCount, aliasCount int
	if err := fixture.db.QueryRow("SELECT COUNT(*) FROM images").Scan(&imageCount); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.QueryRow("SELECT COUNT(*) FROM image_aliases").Scan(&aliasCount); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(filepath.Join(fixture.dataDirectory, "images"))
	if err != nil {
		t.Fatal(err)
	}
	conflict := fixture.importImage(nil, "", both.Token, imported.Alias.Path, "public", data)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("duplicate alias import status = %d, body = %s", conflict.Code, conflict.Body.String())
	}
	var afterImages, afterAliases int
	if err := fixture.db.QueryRow("SELECT COUNT(*) FROM images").Scan(&afterImages); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.QueryRow("SELECT COUNT(*) FROM image_aliases").Scan(&afterAliases); err != nil {
		t.Fatal(err)
	}
	afterFiles, err := os.ReadDir(filepath.Join(fixture.dataDirectory, "images"))
	if err != nil {
		t.Fatal(err)
	}
	if afterImages != imageCount || afterAliases != aliasCount || len(afterFiles) != len(files) {
		t.Fatal("duplicate alias import left persistent state")
	}
}

func (f *phaseTwoFixture) importImage(cookies []*http.Cookie, csrfToken, bearer, alias, visibility string, data []byte) *httptest.ResponseRecorder {
	f.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.jpg")
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		f.t.Fatal(err)
	}
	if err := writer.WriteField("alias", alias); err != nil {
		f.t.Fatal(err)
	}
	if visibility != "" {
		if err := writer.WriteField("visibility", visibility); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		f.t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/imports", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	addAuthentication(request, cookies, csrfToken, bearer)
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, request)
	return response
}
