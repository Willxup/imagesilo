package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestPhaseSevenRepeatedLifecycleLeavesNoTemporaryFiles(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	for index := 0; index < 4; index++ {
		image := fixture.uploadBytes(
			cookies, csrfToken, "public", "", fmt.Sprintf("lifecycle-%d.jpg", index), phaseTwoJPEG(t),
		)
		response := fixture.request(http.MethodDelete, "/api/v1/images/"+image.ID, nil, cookies, csrfToken, "")
		if response.Code != http.StatusOK {
			t.Fatalf("delete %s status = %d, body = %s", image.ID, response.Code, response.Body.String())
		}
	}

	inspection := fixture.request(http.MethodPost, "/api/v1/maintenance/inspect", nil, cookies, csrfToken, "")
	if inspection.Code != http.StatusOK {
		t.Fatalf("inspection status = %d, body = %s", inspection.Code, inspection.Body.String())
	}
	rebuild := fixture.request(http.MethodPost, "/api/v1/maintenance/rebuild", nil, cookies, csrfToken, "")
	if rebuild.Code != http.StatusOK {
		t.Fatalf("rebuild status = %d, body = %s", rebuild.Code, rebuild.Body.String())
	}

	for _, relative := range []string{"tmp", "images", filepath.Join("cache", "thumbnails")} {
		entries, err := os.ReadDir(filepath.Join(fixture.dataDirectory, relative))
		if err != nil {
			t.Fatalf("ReadDir(%s): %v", relative, err)
		}
		if len(entries) != 0 {
			t.Fatalf("%s retained %d entries after repeated upload/delete/inspect/rebuild", relative, len(entries))
		}
	}
}
