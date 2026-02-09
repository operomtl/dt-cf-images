//go:build conformance

package conformance

import (
	"net/http"
	"testing"
)

func TestKeys_Create_ResponseFields(t *testing.T) {
	result := createKeyAndCleanup(t, "test-key-fields")

	assertField[string](t, result, "name")
	assertField[string](t, result, "value")

	// created_at should NOT be present
	assertFieldAbsent(t, result, "created_at")
}

func TestKeys_List_ResponseShape(t *testing.T) {
	createKeyAndCleanup(t, "list-key-a")
	createKeyAndCleanup(t, "list-key-b")

	status, raw := doJSON(t, "GET", apiURL("/v1/keys"), nil)
	if status != http.StatusOK {
		t.Fatalf("list keys returned %d", status)
	}

	result := assertField[map[string]any](t, raw, "result")
	keys := assertField[[]any](t, result, "keys")

	if len(keys) < 2 {
		t.Errorf("expected at least 2 keys, got %d", len(keys))
	}

	for i, k := range keys {
		keyObj, ok := k.(map[string]any)
		if !ok {
			t.Errorf("keys[%d] should be object", i)
			continue
		}
		assertField[string](t, keyObj, "name")
		assertField[string](t, keyObj, "value")
		assertFieldAbsent(t, keyObj, "created_at")
	}
}

func TestKeys_DeleteLast_AutoCreatesDefault(t *testing.T) {
	skipOnRealAPI(t) // destructive test

	// Create a single key
	keyName := "only-key-for-auto-create"
	status, _ := doJSON(t, "PUT", apiURL("/v1/keys/"+keyName), nil)
	if status != http.StatusOK {
		t.Fatalf("create key returned %d", status)
	}

	// Delete it
	status, _ = doJSON(t, "DELETE", apiURL("/v1/keys/"+keyName), nil)
	if status != http.StatusOK {
		t.Fatalf("delete key returned %d", status)
	}

	// List should have auto-created "default" key
	status, raw := doJSON(t, "GET", apiURL("/v1/keys"), nil)
	if status != http.StatusOK {
		t.Fatalf("list keys returned %d", status)
	}

	result := raw["result"].(map[string]any)
	keys := result["keys"].([]any)

	found := false
	for _, k := range keys {
		keyObj := k.(map[string]any)
		if keyObj["name"] == "default" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected auto-created 'default' key after deleting last key")
	}

	// Cleanup the default key
	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", apiURL("/v1/keys/default"), nil)
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	})
}
