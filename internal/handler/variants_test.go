package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/leca/dt-cloudflare-images/internal/api"
	"github.com/leca/dt-cloudflare-images/internal/config"
	"github.com/leca/dt-cloudflare-images/internal/database"
	"github.com/leca/dt-cloudflare-images/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAccountID = "test-account-001"

func setupVariantTestRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/accounts/{account_id}/images/v1/variants", func(r chi.Router) {
		r.Use(api.AccountIDMiddleware)
		r.Post("/", h.CreateVariant)
		r.Get("/", h.ListVariants)
		r.Get("/{variant_id}", h.GetVariant)
		r.Patch("/{variant_id}", h.UpdateVariant)
		r.Delete("/{variant_id}", h.DeleteVariant)
	})
	return r
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := database.NewSQLiteDB("file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	tmpDir := t.TempDir()
	store := storage.NewFileSystem(tmpDir)

	return &Handler{
		DB:     db,
		Store:  store,
		Config: &config.Config{},
	}
}

func createVariantJSON(id, fit string, width, height int) string {
	return fmt.Sprintf(`{
		"id": %q,
		"options": {"fit": %q, "width": %d, "height": %d, "metadata": "none"},
		"neverRequireSignedURLs": false
	}`, id, fit, width, height)
}

func TestCreateVariant(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	body := createVariantJSON("hero", "scale-down", 1920, 1080)
	req := httptest.NewRequest(http.MethodPost, "/accounts/"+testAccountID+"/images/v1/variants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Empty(t, resp.Errors)

	// Verify result contains variant data.
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "hero", result["id"])

	opts, ok := result["options"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "scale-down", opts["fit"])
	assert.Equal(t, float64(1920), opts["width"])
	assert.Equal(t, float64(1080), opts["height"])
}

func TestCreateVariant_InvalidFit(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	body := createVariantJSON("bad-fit", "stretch", 200, 200)
	req := httptest.NewRequest(http.MethodPost, "/accounts/"+testAccountID+"/images/v1/variants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Errors)
}

func TestCreateVariant_MaxLimit(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	// Create 100 variants (the maximum).
	for i := 0; i < 100; i++ {
		body := createVariantJSON(fmt.Sprintf("variant-%03d", i), "scale-down", 100, 100)
		req := httptest.NewRequest(http.MethodPost, "/accounts/"+testAccountID+"/images/v1/variants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "failed to create variant %d", i)
	}

	// The 101st should be rejected.
	body := createVariantJSON("variant-overflow", "scale-down", 100, 100)
	req := httptest.NewRequest(http.MethodPost, "/accounts/"+testAccountID+"/images/v1/variants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Len(t, resp.Errors, 1)
	assert.Contains(t, resp.Errors[0].Message, "maximum number of variants reached")
}

func TestListVariants(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	// Create several variants.
	names := []string{"thumb", "medium", "large"}
	for _, name := range names {
		body := createVariantJSON(name, "contain", 200, 200)
		req := httptest.NewRequest(http.MethodPost, "/accounts/"+testAccountID+"/images/v1/variants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	// List all variants.
	req := httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/variants", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	variants, ok := result["variants"].(map[string]interface{})
	require.True(t, ok)
	assert.Len(t, variants, 3)

	// Each key should correspond to a variant name.
	for _, name := range names {
		_, exists := variants[name]
		assert.True(t, exists, "variant %q should be in the list", name)
	}
}

func TestGetVariant(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	// Create a variant first.
	body := createVariantJSON("hero", "cover", 800, 600)
	req := httptest.NewRequest(http.MethodPost, "/accounts/"+testAccountID+"/images/v1/variants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Get it.
	req = httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/variants/hero", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "hero", result["id"])
}

func TestGetVariant_NotFound(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/variants/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
}

func TestUpdateVariant(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	// Create a variant.
	body := createVariantJSON("updatable", "scale-down", 400, 300)
	req := httptest.NewRequest(http.MethodPost, "/accounts/"+testAccountID+"/images/v1/variants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Update it.
	updateBody := `{"options": {"fit": "cover", "width": 800, "height": 600, "metadata": "keep"}, "neverRequireSignedURLs": true}`
	req = httptest.NewRequest(http.MethodPatch, "/accounts/"+testAccountID+"/images/v1/variants/updatable", bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	opts, ok := result["options"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "cover", opts["fit"])
	assert.Equal(t, float64(800), opts["width"])
	assert.Equal(t, float64(600), opts["height"])
	assert.Equal(t, "keep", opts["metadata"])
	assert.Equal(t, true, result["neverRequireSignedURLs"])
}

func TestDeleteVariant(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	// Create a variant.
	body := createVariantJSON("deletable", "pad", 100, 100)
	req := httptest.NewRequest(http.MethodPost, "/accounts/"+testAccountID+"/images/v1/variants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Delete it.
	req = httptest.NewRequest(http.MethodDelete, "/accounts/"+testAccountID+"/images/v1/variants/deletable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify it's gone.
	req = httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/variants/deletable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteVariant_NotFound(t *testing.T) {
	h := newTestHandler(t)
	router := setupVariantTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/accounts/"+testAccountID+"/images/v1/variants/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
}
