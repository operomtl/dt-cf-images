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

// apiURL builds a full URL for the given API path suffix.
// path should start with "/" e.g. "/v1" or "/v2/direct_upload"
func apiURL(path string) string {
	return strings.TrimRight(baseURL, "/") + "/accounts/" + accountID + "/images" + path
}

// doRequest performs an HTTP request and returns the response.
func doRequest(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

// doJSON performs an HTTP request and returns the decoded JSON as map[string]any.
func doJSON(t *testing.T, method, url string, body io.Reader) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp := doRequest(t, req)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal JSON: %v\nbody: %s", err, string(data))
	}
	return resp.StatusCode, raw
}

// doMultipartUpload performs a multipart file upload and returns the decoded JSON.
func doMultipartUpload(t *testing.T, url string, fileContent []byte, fileName string) (int, map[string]any) {
	t.Helper()
	body, contentType := multipartBody(t, "file", fileName, fileContent)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("Content-Type", contentType)
	resp := doRequest(t, req)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal JSON: %v\nbody: %s", err, string(data))
	}
	return resp.StatusCode, raw
}

// multipartBody builds a multipart form body with a single file field.
func multipartBody(t *testing.T, fieldName, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

// assertEnvelopeShape validates the universal Cloudflare response envelope structure.
func assertEnvelopeShape(t *testing.T, raw map[string]any) {
	t.Helper()

	// success must be a bool
	success, ok := raw["success"]
	if !ok {
		t.Error("envelope missing 'success' field")
	} else if _, ok := success.(bool); !ok {
		t.Errorf("'success' should be bool, got %T", success)
	}

	// errors must be an array of objects with code+message
	errors, ok := raw["errors"]
	if !ok {
		t.Error("envelope missing 'errors' field")
	} else if errArr, ok := errors.([]any); ok {
		for i, e := range errArr {
			errObj, ok := e.(map[string]any)
			if !ok {
				t.Errorf("errors[%d] should be object, got %T", i, e)
				continue
			}
			if _, ok := errObj["code"]; !ok {
				t.Errorf("errors[%d] missing 'code'", i)
			}
			if _, ok := errObj["message"]; !ok {
				t.Errorf("errors[%d] missing 'message'", i)
			}
		}
	} else {
		t.Errorf("'errors' should be array, got %T", errors)
	}

	// messages must be an array of objects with code+message
	messages, ok := raw["messages"]
	if !ok {
		t.Error("envelope missing 'messages' field")
	} else if msgArr, ok := messages.([]any); ok {
		for i, m := range msgArr {
			msgObj, ok := m.(map[string]any)
			if !ok {
				t.Errorf("messages[%d] should be object, got %T", i, m)
				continue
			}
			if _, ok := msgObj["code"]; !ok {
				t.Errorf("messages[%d] missing 'code'", i)
			}
			if _, ok := msgObj["message"]; !ok {
				t.Errorf("messages[%d] missing 'message'", i)
			}
		}
	} else {
		t.Errorf("'messages' should be array, got %T", messages)
	}
}

// assertField validates a field exists in an object and has the expected Go type.
// Returns the typed value.
func assertField[T any](t *testing.T, obj map[string]any, field string) T {
	t.Helper()
	val, ok := obj[field]
	if !ok {
		var zero T
		t.Errorf("missing field %q", field)
		return zero
	}
	typed, ok := val.(T)
	if !ok {
		var zero T
		t.Errorf("field %q: expected %T, got %T (%v)", field, zero, val, val)
		return zero
	}
	return typed
}

// assertFieldAbsent validates a field does NOT exist in the object.
func assertFieldAbsent(t *testing.T, obj map[string]any, field string) {
	t.Helper()
	if _, ok := obj[field]; ok {
		t.Errorf("field %q should be absent but is present", field)
	}
}

// uploadAndCleanup uploads a test image and registers a cleanup to delete it.
// Returns the raw result object from the upload response.
func uploadAndCleanup(t *testing.T) map[string]any {
	t.Helper()
	status, raw := doMultipartUpload(t, apiURL("/v1"), []byte("test-image-data"), "test.png")
	if status != http.StatusOK {
		t.Fatalf("upload failed with status %d: %v", status, raw)
	}

	result, ok := raw["result"].(map[string]any)
	if !ok {
		t.Fatalf("upload result is not object: %T", raw["result"])
	}

	imageID, ok := result["id"].(string)
	if !ok {
		t.Fatalf("upload result missing id")
	}

	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/"+imageID), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	})

	return result
}

// createVariantAndCleanup creates a variant and registers cleanup.
func createVariantAndCleanup(t *testing.T, id string) map[string]any {
	t.Helper()
	body := fmt.Sprintf(`{"id":%q,"options":{"fit":"scale-down","width":100,"height":100,"metadata":"none"}}`, id)
	status, raw := doJSON(t, "POST", apiURL("/v1/variants"), strings.NewReader(body))
	if status != http.StatusOK {
		t.Fatalf("create variant failed with status %d: %v", status, raw)
	}

	result, ok := raw["result"].(map[string]any)
	if !ok {
		t.Fatalf("variant result is not object: %T", raw["result"])
	}

	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/variants/"+id), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	})

	return result
}

// createKeyAndCleanup creates a signing key and registers cleanup.
func createKeyAndCleanup(t *testing.T, name string) map[string]any {
	t.Helper()
	status, raw := doJSON(t, "PUT", apiURL("/v1/keys/"+name), nil)
	if status != http.StatusOK {
		t.Fatalf("create key failed with status %d: %v", status, raw)
	}

	result, ok := raw["result"].(map[string]any)
	if !ok {
		t.Fatalf("key result is not object: %T", raw["result"])
	}

	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/keys/"+name), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	})

	return result
}

// skipOnRealAPI skips the test when running against the real Cloudflare API.
func skipOnRealAPI(t *testing.T) {
	t.Helper()
	if isRealAPI {
		t.Skip("skipping on real API")
	}
}

// skipOnDigitalTwin skips the test when running against the digital twin.
func skipOnDigitalTwin(t *testing.T) {
	t.Helper()
	if !isRealAPI {
		t.Skip("skipping on digital twin")
	}
}
