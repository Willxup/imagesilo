package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Willxup/imagesilo/internal/apitoken"
)

func TestPhaseFiveImageManagementPaginationDetailAndBatchOperations(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	first := fixture.uploadBytes(cookies, csrfToken, "public", "", "first-searchable.jpg", phaseTwoJPEG(t))
	second := fixture.uploadBytes(cookies, csrfToken, "private", "", "second.png", phaseTwoJPEG(t))
	third := fixture.uploadBytes(cookies, csrfToken, "public", "", "third.jpg", phaseTwoJPEG(t))

	alias := fixture.request(http.MethodPost, "/api/v1/aliases", map[string]any{
		"path": "/legacy/first-searchable.jpg", "imageId": first.ID, "source": "phase-five-test",
	}, cookies, csrfToken, "")
	if alias.Code != http.StatusCreated {
		t.Fatalf("create alias status = %d, body = %s", alias.Code, alias.Body.String())
	}

	pageOne := fixture.request(http.MethodGet, "/api/v1/images?limit=2", nil, cookies, "", "")
	if pageOne.Code != http.StatusOK {
		t.Fatalf("first page status = %d, body = %s", pageOne.Code, pageOne.Body.String())
	}
	var firstPage imageListResponse
	if err := json.Unmarshal(pageOne.Body.Bytes(), &firstPage); err != nil || len(firstPage.Items) != 2 || firstPage.NextCursor == "" {
		t.Fatalf("first page = %+v, error = %v", firstPage, err)
	}
	pageTwo := fixture.request(http.MethodGet, "/api/v1/images?limit=2&cursor="+url.QueryEscape(firstPage.NextCursor), nil, cookies, "", "")
	var secondPage imageListResponse
	if err := json.Unmarshal(pageTwo.Body.Bytes(), &secondPage); pageTwo.Code != http.StatusOK || err != nil || len(secondPage.Items) != 1 || secondPage.NextCursor != "" {
		t.Fatalf("second page status = %d, value = %+v, error = %v", pageTwo.Code, secondPage, err)
	}
	search := fixture.request(http.MethodGet, "/api/v1/images?q="+url.QueryEscape("legacy/first-searchable"), nil, cookies, "", "")
	var searchPage imageListResponse
	if err := json.Unmarshal(search.Body.Bytes(), &searchPage); search.Code != http.StatusOK || err != nil || len(searchPage.Items) != 1 || searchPage.Items[0].ID != first.ID {
		t.Fatalf("alias search status = %d, value = %+v, error = %v", search.Code, searchPage, err)
	}
	filtered := fixture.request(http.MethodGet, "/api/v1/images?visibility=private&minWidth=1&uploadedVia=admin", nil, cookies, "", "")
	var filteredPage imageListResponse
	if err := json.Unmarshal(filtered.Body.Bytes(), &filteredPage); filtered.Code != http.StatusOK || err != nil || len(filteredPage.Items) != 1 || filteredPage.Items[0].ID != second.ID {
		t.Fatalf("filtered list status = %d, value = %+v, error = %v", filtered.Code, filteredPage, err)
	}
	detail := fixture.request(http.MethodGet, "/api/v1/images/"+first.ID, nil, cookies, "", "")
	var details imageDetailResponse
	if err := json.Unmarshal(detail.Body.Bytes(), &details); detail.Code != http.StatusOK || err != nil || details.ID != first.ID || len(details.Aliases) != 1 {
		t.Fatalf("detail status = %d, value = %+v, error = %v", detail.Code, details, err)
	}

	visibility := fixture.request(http.MethodPatch, "/api/v1/images/batch-visibility", map[string]any{
		"imageIds": []string{first.ID, third.ID}, "visibility": "private",
	}, cookies, csrfToken, "")
	var visibilityResult batchOperationResponse
	if err := json.Unmarshal(visibility.Body.Bytes(), &visibilityResult); visibility.Code != http.StatusOK || err != nil || len(visibilityResult.Items) != 2 {
		t.Fatalf("batch visibility status = %d, value = %+v, error = %v", visibility.Code, visibilityResult, err)
	}
	for _, item := range visibilityResult.Items {
		if item.Status != "updated" {
			t.Fatalf("batch visibility item = %+v", item)
		}
	}

	wrongToken := fixture.createToken(cookies, csrfToken, "wrong scope", []apitoken.Scope{apitoken.ScopeImagesUpload})
	deleteToken := fixture.createToken(cookies, csrfToken, "delete scope", []apitoken.Scope{apitoken.ScopeImagesDelete})
	if response := fixture.request(http.MethodDelete, "/api/v1/images/"+second.ID, nil, nil, "", wrongToken.Token); response.Code != http.StatusForbidden {
		t.Fatalf("wrong-scope delete status = %d", response.Code)
	}
	batchDelete := fixture.request(http.MethodPost, "/api/v1/images/batch-delete", map[string]any{
		"imageIds": []string{second.ID, "019c1234-5678-7abc-8def-0123456789ff"},
	}, nil, "", deleteToken.Token)
	var deleteResult batchOperationResponse
	if err := json.Unmarshal(batchDelete.Body.Bytes(), &deleteResult); batchDelete.Code != http.StatusOK || err != nil || len(deleteResult.Items) != 2 {
		t.Fatalf("batch delete status = %d, value = %+v, error = %v", batchDelete.Code, deleteResult, err)
	}
	if deleteResult.Items[0].Status != "deleted" || deleteResult.Items[1].Status != "not_found" || deleteResult.Items[1].ErrorCode != "image_not_found" {
		t.Fatalf("batch delete results = %+v", deleteResult.Items)
	}
	if response := fixture.request(http.MethodGet, second.StandardURL, nil, nil, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("deleted image delivery status = %d", response.Code)
	}
}

func TestPhaseFiveOverviewInspectionAndManualRebuild(t *testing.T) {
	fixture := newPhaseTwoFixture(t)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	image := fixture.uploadBytes(cookies, csrfToken, "public", "", "overview.jpg", phaseTwoJPEG(t))

	overview := fixture.request(http.MethodGet, "/api/v1/overview", nil, cookies, "", "")
	var firstOverview overviewResponse
	if err := json.Unmarshal(overview.Body.Bytes(), &firstOverview); overview.Code != http.StatusOK || err != nil ||
		firstOverview.ImageCount != 1 || firstOverview.Indexes.Images != 1 || !firstOverview.IndexConsistent {
		t.Fatalf("overview status = %d, value = %+v, error = %v", overview.Code, firstOverview, err)
	}
	if err := os.Remove(filepath.Join(fixture.dataDirectory, "images", image.ID)); err != nil {
		t.Fatalf("remove image before inspection: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixture.dataDirectory, "images", "orphan-file"), []byte("orphan"), 0o640); err != nil {
		t.Fatalf("write orphan image: %v", err)
	}
	inspection := fixture.request(http.MethodPost, "/api/v1/maintenance/inspect", nil, cookies, csrfToken, "")
	var inspectionResult inspectionResponse
	if err := json.Unmarshal(inspection.Body.Bytes(), &inspectionResult); inspection.Code != http.StatusOK || err != nil ||
		inspectionResult.MissingImageCount != 1 || inspectionResult.OrphanImageCount != 1 || len(inspectionResult.MissingImageIDs) != 1 {
		t.Fatalf("inspection status = %d, value = %+v, error = %v", inspection.Code, inspectionResult, err)
	}
	rebuild := fixture.request(http.MethodPost, "/api/v1/maintenance/rebuild", nil, cookies, csrfToken, "")
	var rebuildResult rebuildResponse
	if err := json.Unmarshal(rebuild.Body.Bytes(), &rebuildResult); rebuild.Code != http.StatusOK || err != nil ||
		rebuildResult.Images != 0 || rebuildResult.MissingImageCount != 1 {
		t.Fatalf("rebuild status = %d, value = %+v, error = %v", rebuild.Code, rebuildResult, err)
	}
	after := fixture.request(http.MethodGet, "/api/v1/overview", nil, cookies, "", "")
	var finalOverview overviewResponse
	if err := json.Unmarshal(after.Body.Bytes(), &finalOverview); after.Code != http.StatusOK || err != nil || finalOverview.IndexConsistent || finalOverview.MissingImageCount != 1 ||
		finalOverview.LastInspection == nil || finalOverview.LastRebuild == nil {
		t.Fatalf("final overview status = %d, value = %+v, error = %v", after.Code, finalOverview, err)
	}
}
