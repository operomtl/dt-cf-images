//go:build conformance

package conformance

import (
	"net/http"
	"testing"
)

func TestEnvelope_SuccessShape(t *testing.T) {
	status, raw := doMultipartUpload(t, apiURL("/v1"), []byte("test-image-data"), "test.png")
	if status != http.StatusOK {
		t.Fatalf("upload failed with status %d: %v", status, raw)
	}

	// Register cleanup for the uploaded image
	result, ok := raw["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not an object: %T", raw["result"])
	}
	imageID, ok := result["id"].(string)
	if !ok {
		t.Fatalf("result missing id")
	}
	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/"+imageID), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	})

	assertEnvelopeShape(t, raw)

	if raw["result"] == nil {
		t.Error("result should not be nil on success")
	}
}

func TestEnvelope_ErrorShape(t *testing.T) {
	status, raw := doJSON(t, "GET", apiURL("/v1/nonexistent-id"), nil)
	if status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", status)
	}

	assertEnvelopeShape(t, raw)

	success, ok := raw["success"].(bool)
	if !ok {
		t.Fatal("success field is not bool")
	}
	if success {
		t.Error("success should be false for error responses")
	}

	if raw["result"] != nil {
		t.Error("result should be nil for error responses")
	}

	errors, ok := raw["errors"].([]any)
	if !ok {
		t.Fatal("errors is not an array")
	}
	if len(errors) == 0 {
		t.Error("errors array should be non-empty for error responses")
	}
}

func TestEnvelope_PaginatedShape(t *testing.T) {
	status, raw := doJSON(t, "GET", apiURL("/v1"), nil)
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}

	assertEnvelopeShape(t, raw)

	resultInfo, ok := raw["result_info"].(map[string]any)
	if !ok {
		t.Fatalf("result_info should be an object, got %T", raw["result_info"])
	}

	assertField[float64](t, resultInfo, "page")
	assertField[float64](t, resultInfo, "per_page")
	assertField[float64](t, resultInfo, "count")
	assertField[float64](t, resultInfo, "total_count")
}
