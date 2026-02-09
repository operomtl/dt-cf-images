//go:build conformance

package conformance

import (
	"net/http"
	"strings"
	"testing"
)

func TestVariant_Create_ResponseFields(t *testing.T) {
	result := createVariantAndCleanup(t, "test-variant-fields")

	assertField[string](t, result, "id")
	options := assertField[map[string]any](t, result, "options")
	assertField[string](t, options, "fit")
	assertField[float64](t, options, "width")
	assertField[float64](t, options, "height")
	assertField[string](t, options, "metadata")

	// neverRequireSignedURLs should be present
	if _, ok := result["neverRequireSignedURLs"]; !ok {
		t.Error("missing 'neverRequireSignedURLs' field")
	}
}

func TestVariant_List_MapFormat(t *testing.T) {
	createVariantAndCleanup(t, "map-test-a")
	createVariantAndCleanup(t, "map-test-b")

	status, raw := doJSON(t, "GET", apiURL("/v1/variants"), nil)
	if status != http.StatusOK {
		t.Fatalf("list variants returned %d", status)
	}

	result := assertField[map[string]any](t, raw, "result")
	variants := assertField[map[string]any](t, result, "variants")

	// Should be a map keyed by variant ID, not an array
	if _, ok := variants["map-test-a"]; !ok {
		t.Error("variants should contain 'map-test-a'")
	}
	if _, ok := variants["map-test-b"]; !ok {
		t.Error("variants should contain 'map-test-b'")
	}
}

func TestVariant_CRUD_Lifecycle(t *testing.T) {
	variantID := "lifecycle-test"

	// Create
	body := `{"id":"lifecycle-test","options":{"fit":"scale-down","width":200,"height":200,"metadata":"none"}}`
	status, raw := doJSON(t, "POST", apiURL("/v1/variants"), strings.NewReader(body))
	if status != http.StatusOK {
		t.Fatalf("create variant returned %d", status)
	}

	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/variants/"+variantID), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Get
	status, raw = doJSON(t, "GET", apiURL("/v1/variants/"+variantID), nil)
	if status != http.StatusOK {
		t.Fatalf("get variant returned %d", status)
	}
	result := assertField[map[string]any](t, raw, "result")
	if result["id"] != variantID {
		t.Errorf("variant id: got %v, want %s", result["id"], variantID)
	}

	// Update
	updateBody := `{"options":{"width":400}}`
	status, raw = doJSON(t, "PATCH", apiURL("/v1/variants/"+variantID), strings.NewReader(updateBody))
	if status != http.StatusOK {
		t.Fatalf("update variant returned %d", status)
	}
	result = assertField[map[string]any](t, raw, "result")
	opts := result["options"].(map[string]any)
	if opts["width"] != float64(400) {
		t.Errorf("width after update: got %v, want 400", opts["width"])
	}

	// Delete
	status, _ = doJSON(t, "DELETE", apiURL("/v1/variants/"+variantID), nil)
	if status != http.StatusOK {
		t.Fatalf("delete variant returned %d", status)
	}

	// Verify 404
	status, _ = doJSON(t, "GET", apiURL("/v1/variants/"+variantID), nil)
	if status != http.StatusNotFound {
		t.Errorf("get after delete: got %d, want 404", status)
	}
}
