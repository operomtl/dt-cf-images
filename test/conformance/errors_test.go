//go:build conformance

package conformance

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestError_404_Format(t *testing.T) {
	status, raw := doJSON(t, "GET", apiURL("/v1/nonexistent-id"), nil)
	if status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", status)
	}

	assertEnvelopeShape(t, raw)

	errors, ok := raw["errors"].([]any)
	if !ok {
		t.Fatal("errors is not an array")
	}
	if len(errors) == 0 {
		t.Fatal("errors array should be non-empty")
	}

	errObj, ok := errors[0].(map[string]any)
	if !ok {
		t.Fatal("errors[0] is not an object")
	}

	// code should be a number (float64 from JSON)
	if _, ok := errObj["code"].(float64); !ok {
		t.Errorf("errors[0].code should be numeric, got %T", errObj["code"])
	}

	// message should be a string
	if _, ok := errObj["message"].(string); !ok {
		t.Errorf("errors[0].message should be string, got %T", errObj["message"])
	}
}

func TestError_400_Format(t *testing.T) {
	req, err := http.NewRequest("POST", apiURL("/v1"), strings.NewReader("not-multipart"))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")

	resp := doRequest(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	// Read and parse JSON body
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(respData, &data); err != nil {
		t.Fatalf("unmarshal JSON: %v\nbody: %s", err, string(respData))
	}

	assertEnvelopeShape(t, data)
}

func TestError_409_Format(t *testing.T) {
	variantID := "test-dup-variant-409"
	createVariantAndCleanup(t, variantID)

	// Try to create the same variant again
	body := `{"id":"` + variantID + `","options":{"fit":"scale-down","width":100,"height":100,"metadata":"none"}}`
	status, raw := doJSON(t, "POST", apiURL("/v1/variants"), strings.NewReader(body))
	if status != http.StatusConflict {
		t.Errorf("expected status 409, got %d", status)
	}

	assertEnvelopeShape(t, raw)
}
