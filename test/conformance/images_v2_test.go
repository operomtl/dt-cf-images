//go:build conformance

package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestV2_List_ResponseShape(t *testing.T) {
	uploadAndCleanup(t) // ensure at least one image

	status, raw := doJSON(t, "GET", apiURL("/v2"), nil)
	if status != http.StatusOK {
		t.Fatalf("v2 list returned %d", status)
	}

	result := assertField[map[string]any](t, raw, "result")
	assertField[[]any](t, result, "images")
	// continuation_token should exist (may be empty string)
	if _, ok := result["continuation_token"]; !ok {
		t.Error("missing continuation_token field")
	}
}

func TestV2_CursorPagination_NoDuplicates(t *testing.T) {
	// Upload 5 images
	for i := 0; i < 5; i++ {
		uploadAndCleanup(t)
	}

	allIDs := make(map[string]bool)
	nextToken := ""

	for pages := 0; pages < 10; pages++ {
		url := apiURL("/v2?per_page=2")
		if nextToken != "" {
			url += "&continuation_token=" + nextToken
		}

		status, raw := doJSON(t, "GET", url, nil)
		if status != http.StatusOK {
			t.Fatalf("v2 list returned %d", status)
		}

		result := raw["result"].(map[string]any)
		images := result["images"].([]any)

		for _, img := range images {
			imgObj := img.(map[string]any)
			id := imgObj["id"].(string)
			if allIDs[id] {
				t.Errorf("duplicate image ID: %s", id)
			}
			allIDs[id] = true
		}

		token, _ := result["continuation_token"].(string)
		if token == "" {
			break
		}
		nextToken = token
	}

	if len(allIDs) < 5 {
		t.Errorf("expected at least 5 unique images, got %d", len(allIDs))
	}
}

func TestV2_DirectUpload_ResponseShape(t *testing.T) {
	body := `{}`
	status, raw := doJSON(t, "POST", apiURL("/v2/direct_upload"), strings.NewReader(body))
	if status != http.StatusOK {
		t.Fatalf("direct upload create returned %d", status)
	}

	result := assertField[map[string]any](t, raw, "result")
	id := assertField[string](t, result, "id")
	assertField[string](t, result, "uploadURL")

	// result_info should be present and null
	if _, ok := raw["result_info"]; !ok {
		t.Error("missing result_info field (should be null)")
	} else if raw["result_info"] != nil {
		t.Errorf("result_info should be null, got %v", raw["result_info"])
	}

	// Clean up: we can't easily delete a pending direct upload, but the image
	// doesn't exist yet so it's fine. Register cleanup for the image ID in case
	// something uploads to it.
	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/"+id), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	})
}

func TestV2_DirectUpload_FullFlow(t *testing.T) {
	// Step 1: Create direct upload
	body := `{}`
	status, raw := doJSON(t, "POST", apiURL("/v2/direct_upload"), strings.NewReader(body))
	if status != http.StatusOK {
		t.Fatalf("create direct upload returned %d", status)
	}

	result := raw["result"].(map[string]any)
	uploadID := result["id"].(string)
	uploadURL := result["uploadURL"].(string)

	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/"+uploadID), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	})

	// Step 2: Upload file to the upload URL (no auth needed)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "direct-test.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fw.Write([]byte("direct-upload-content"))
	mw.Close()

	req, _ := http.NewRequest("POST", uploadURL, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp := doRequest(t, req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload to direct URL returned %d", resp.StatusCode)
	}

	respData, _ := io.ReadAll(resp.Body)
	var uploadRaw map[string]any
	if err := json.Unmarshal(respData, &uploadRaw); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}

	uploadResult := uploadRaw["result"].(map[string]any)

	// Verify image ID matches upload ID
	if uploadResult["id"] != uploadID {
		t.Errorf("image ID should match upload ID: got %v, want %s", uploadResult["id"], uploadID)
	}

	// Verify draft=true for direct upload
	draft, ok := uploadResult["draft"]
	if !ok {
		t.Error("missing 'draft' field in direct upload response")
	} else if draft != true {
		t.Errorf("draft should be true for direct upload, got %v", draft)
	}
}

// Silence unused import warnings.
var _ = fmt.Sprintf
