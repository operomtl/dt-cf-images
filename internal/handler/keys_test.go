package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/leca/dt-cloudflare-images/internal/api"
	"github.com/leca/dt-cloudflare-images/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupKeysTestRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/accounts/{account_id}/images/v1/keys", func(r chi.Router) {
		r.Use(api.AccountIDMiddleware)
		r.Get("/", h.ListSigningKeys)
		r.Put("/{signing_key_name}", h.CreateSigningKey)
		r.Delete("/{signing_key_name}", h.DeleteSigningKey)
	})
	return r
}

func TestCreateSigningKey(t *testing.T) {
	h := newTestHandler(t)
	router := setupKeysTestRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/accounts/"+testAccountID+"/images/v1/keys/my-key", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "my-key", result["name"])
	assert.NotEmpty(t, result["value"], "key value should be a generated UUID")
}

func TestListSigningKeys(t *testing.T) {
	h := newTestHandler(t)
	router := setupKeysTestRouter(h)

	// Create several keys.
	keyNames := []string{"key-alpha", "key-beta", "key-gamma"}
	for _, name := range keyNames {
		req := httptest.NewRequest(http.MethodPut, "/accounts/"+testAccountID+"/images/v1/keys/"+name, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	// List all keys.
	req := httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/keys/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	keysRaw, ok := result["keys"].([]interface{})
	require.True(t, ok)
	assert.Len(t, keysRaw, 3)

	// Verify each key has a name and value.
	names := make(map[string]bool)
	for _, k := range keysRaw {
		keyObj, ok := k.(map[string]interface{})
		require.True(t, ok)
		name, ok := keyObj["name"].(string)
		require.True(t, ok)
		names[name] = true
		assert.NotEmpty(t, keyObj["value"], "key should include value")
	}
	for _, name := range keyNames {
		assert.True(t, names[name], "key %q should be in the list", name)
	}
}

func TestDeleteSigningKey(t *testing.T) {
	h := newTestHandler(t)
	router := setupKeysTestRouter(h)

	// Create two keys so deleting one doesn't trigger auto-create.
	for _, name := range []string{"key-one", "key-two"} {
		req := httptest.NewRequest(http.MethodPut, "/accounts/"+testAccountID+"/images/v1/keys/"+name, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	// Delete one key.
	req := httptest.NewRequest(http.MethodDelete, "/accounts/"+testAccountID+"/images/v1/keys/key-one", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify only one key remains.
	req = httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/keys/", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var listResp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &listResp)
	require.NoError(t, err)
	result, ok := listResp.Result.(map[string]interface{})
	require.True(t, ok)
	keysRaw, ok := result["keys"].([]interface{})
	require.True(t, ok)
	assert.Len(t, keysRaw, 1)
}

func TestDeleteSigningKey_NotFound(t *testing.T) {
	h := newTestHandler(t)
	router := setupKeysTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/accounts/"+testAccountID+"/images/v1/keys/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
}

func TestDeleteSigningKey_LastKey_AutoCreatesDefault(t *testing.T) {
	h := newTestHandler(t)
	router := setupKeysTestRouter(h)

	// Create a single key.
	req := httptest.NewRequest(http.MethodPut, "/accounts/"+testAccountID+"/images/v1/keys/only-key", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Delete it -- should trigger auto-creation of "default".
	req = httptest.NewRequest(http.MethodDelete, "/accounts/"+testAccountID+"/images/v1/keys/only-key", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// List keys -- should find exactly one "default" key.
	req = httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/keys/", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	keysRaw, ok := result["keys"].([]interface{})
	require.True(t, ok)
	require.Len(t, keysRaw, 1)

	defaultKey, ok := keysRaw[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "default", defaultKey["name"])
	assert.NotEmpty(t, defaultKey["value"])

	// Verify the auto-created key is actually a valid SigningKey in DB.
	keys, err := h.DB.ListSigningKeys(testAccountID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "default", keys[0].Name)
	assert.Equal(t, model.SigningKey{
		Name:      keys[0].Name,
		Value:     keys[0].Value,
		AccountID: keys[0].AccountID,
		CreatedAt: keys[0].CreatedAt,
	}, *keys[0])
}
