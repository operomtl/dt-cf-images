//go:build conformance

package conformance

import (
	"net/http"
	"testing"
)

func TestV1_List_ResultWrappedInImages(t *testing.T) {
	uploadAndCleanup(t) // ensure at least one image exists

	status, raw := doJSON(t, "GET", apiURL("/v1"), nil)
	if status != http.StatusOK {
		t.Fatalf("list returned %d", status)
	}

	result := assertField[map[string]any](t, raw, "result")

	// result should be an object with "images" key
	images := assertField[[]any](t, result, "images")
	if len(images) == 0 {
		t.Error("images array should not be empty (we uploaded one)")
	}

	// Each image should be an object
	for i, img := range images {
		if _, ok := img.(map[string]any); !ok {
			t.Errorf("images[%d] should be object, got %T", i, img)
		}
	}
}

func TestV1_List_DefaultPerPage1000(t *testing.T) {
	status, raw := doJSON(t, "GET", apiURL("/v1"), nil)
	if status != http.StatusOK {
		t.Fatalf("list returned %d", status)
	}

	resultInfo := assertField[map[string]any](t, raw, "result_info")
	perPage := assertField[float64](t, resultInfo, "per_page")

	if perPage != 1000 {
		t.Errorf("default per_page: got %v, want 1000", perPage)
	}
}

func TestV1_List_Pagination(t *testing.T) {
	// Upload 3 images
	uploadAndCleanup(t)
	uploadAndCleanup(t)
	uploadAndCleanup(t)

	// Page 1 with per_page=2
	status, raw := doJSON(t, "GET", apiURL("/v1?page=1&per_page=2"), nil)
	if status != http.StatusOK {
		t.Fatalf("list page 1 returned %d", status)
	}

	result := assertField[map[string]any](t, raw, "result")
	images := assertField[[]any](t, result, "images")
	if len(images) != 2 {
		t.Errorf("page 1 should have 2 images, got %d", len(images))
	}

	resultInfo := assertField[map[string]any](t, raw, "result_info")
	if resultInfo["per_page"] != float64(2) {
		t.Errorf("per_page should be 2, got %v", resultInfo["per_page"])
	}

	// Page 2 should have remaining images
	status, raw = doJSON(t, "GET", apiURL("/v1?page=2&per_page=2"), nil)
	if status != http.StatusOK {
		t.Fatalf("list page 2 returned %d", status)
	}

	result = assertField[map[string]any](t, raw, "result")
	images = assertField[[]any](t, result, "images")
	if len(images) < 1 {
		t.Error("page 2 should have at least 1 image")
	}
}
