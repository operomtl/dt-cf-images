//go:build conformance

package conformance

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestV1_Upload_ResponseFields(t *testing.T) {
	result := uploadAndCleanup(t)

	// Verify all expected fields exist with correct types
	assertField[string](t, result, "id")
	assertField[string](t, result, "filename")
	assertField[string](t, result, "uploaded")
	assertField[bool](t, result, "requireSignedURLs")
	assertField[string](t, result, "creator")

	// variants should be present (array, possibly empty)
	variants, ok := result["variants"]
	if !ok {
		t.Error("missing 'variants' field")
	} else if _, ok := variants.([]any); !ok {
		t.Errorf("'variants' should be array, got %T", variants)
	}
}

func TestV1_GetImage_MatchesUpload(t *testing.T) {
	uploaded := uploadAndCleanup(t)
	imageID := uploaded["id"].(string)

	status, raw := doJSON(t, "GET", apiURL("/v1/"+imageID), nil)
	if status != http.StatusOK {
		t.Fatalf("GET image returned %d", status)
	}

	result := assertField[map[string]any](t, raw, "result")

	if result["id"] != uploaded["id"] {
		t.Errorf("id mismatch: got %v, want %v", result["id"], uploaded["id"])
	}
	if result["filename"] != uploaded["filename"] {
		t.Errorf("filename mismatch: got %v, want %v", result["filename"], uploaded["filename"])
	}
}

func TestV1_UpdateImage_MetadataAndSignedURLs(t *testing.T) {
	uploaded := uploadAndCleanup(t)
	imageID := uploaded["id"].(string)

	body := `{"metadata":{"env":"test"},"requireSignedURLs":true}`
	status, raw := doJSON(t, "PATCH", apiURL("/v1/"+imageID), strings.NewReader(body))
	if status != http.StatusOK {
		t.Fatalf("PATCH returned %d", status)
	}

	result := assertField[map[string]any](t, raw, "result")

	if result["requireSignedURLs"] != true {
		t.Error("requireSignedURLs should be true after update")
	}
	meta := assertField[map[string]any](t, result, "meta")
	if meta["env"] != "test" {
		t.Errorf("metadata env: got %v, want 'test'", meta["env"])
	}
}

func TestV1_DeleteImage_ThenGet404(t *testing.T) {
	// Upload without using uploadAndCleanup since we're testing delete
	status, raw := doMultipartUpload(t, apiURL("/v1"), []byte("delete-test"), "delete-me.png")
	if status != http.StatusOK {
		t.Fatalf("upload returned %d", status)
	}
	result := raw["result"].(map[string]any)
	imageID := result["id"].(string)

	// Delete
	delStatus, _ := doJSON(t, "DELETE", apiURL("/v1/"+imageID), nil)
	if delStatus != http.StatusOK {
		t.Fatalf("DELETE returned %d", delStatus)
	}

	// GET should 404
	getStatus, _ := doJSON(t, "GET", apiURL("/v1/"+imageID), nil)
	if getStatus != http.StatusNotFound {
		t.Errorf("GET after delete: got %d, want 404", getStatus)
	}
}

func TestV1_GetBlob_MatchesOriginal(t *testing.T) {
	originalContent := []byte("unique-blob-content-12345")
	status, raw := doMultipartUpload(t, apiURL("/v1"), originalContent, "blob-test.bin")
	if status != http.StatusOK {
		t.Fatalf("upload returned %d", status)
	}
	result := raw["result"].(map[string]any)
	imageID := result["id"].(string)

	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/"+imageID), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// GET blob
	req, _ := http.NewRequest("GET", apiURL("/v1/"+imageID+"/blob"), nil)
	req.Header.Set("Authorization", "Bearer "+authToken)
	resp := doRequest(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET blob returned %d", resp.StatusCode)
	}

	blobData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}

	if !bytes.Equal(blobData, originalContent) {
		t.Errorf("blob content mismatch: got %d bytes, want %d bytes", len(blobData), len(originalContent))
	}
}
