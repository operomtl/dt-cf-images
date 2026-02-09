package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/leca/dt-cloudflare-images/internal/api"
	"github.com/leca/dt-cloudflare-images/internal/config"
	"github.com/leca/dt-cloudflare-images/internal/database"
	"github.com/leca/dt-cloudflare-images/internal/model"
	"github.com/leca/dt-cloudflare-images/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStatsTestRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/accounts/{account_id}/images/v1", func(r chi.Router) {
		r.Use(api.AccountIDMiddleware)
		r.Get("/stats", h.GetStats)
	})
	return r
}

// newStatsTestHandler creates a handler with a specific ImageAllowance.
func newStatsTestHandler(t *testing.T, allowance int) *Handler {
	t.Helper()
	db, err := database.NewSQLiteDB("file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	tmpDir := t.TempDir()
	store := storage.NewFileSystem(tmpDir)

	return &Handler{
		DB:    db,
		Store: store,
		Config: &config.Config{
			ImageAllowance: allowance,
		},
	}
}

func TestGetStats_Empty(t *testing.T) {
	h := newStatsTestHandler(t, 100000)
	router := setupStatsTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	countObj, ok := result["count"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(0), countObj["current"])
	assert.Equal(t, float64(100000), countObj["allowed"])
}

func TestGetStats_WithImages(t *testing.T) {
	h := newStatsTestHandler(t, 50000)
	router := setupStatsTestRouter(h)

	// Create images directly in the database.
	for i := 0; i < 5; i++ {
		err := h.DB.CreateImage(&model.Image{
			ID:        uuid.New().String(),
			AccountID: testAccountID,
			Filename:  "test.jpg",
			Uploaded:  time.Now(),
		})
		require.NoError(t, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/accounts/"+testAccountID+"/images/v1/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	countObj, ok := result["count"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(5), countObj["current"])
	assert.Equal(t, float64(50000), countObj["allowed"])
}
