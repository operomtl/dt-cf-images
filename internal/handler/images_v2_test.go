package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/leca/dt-cloudflare-images/internal/api"
	"github.com/leca/dt-cloudflare-images/internal/config"
	"github.com/leca/dt-cloudflare-images/internal/database"
	"github.com/leca/dt-cloudflare-images/internal/handler"
	"github.com/leca/dt-cloudflare-images/internal/model"
	"github.com/leca/dt-cloudflare-images/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupV2TestRouter creates a chi router wired up with the V2 handler routes.
func setupV2TestRouter(h *handler.Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/accounts/{account_id}/images", func(r chi.Router) {
		r.Use(api.AccountIDMiddleware)
		r.Get("/v2", h.ListImagesV2)
		r.Post("/v2/direct_upload", h.CreateDirectUpload)
	})
	r.Post("/upload/{upload_id}", h.HandleDirectUpload)
	return r
}

// setupV2Test creates in-memory DB, temp storage, handler, and router for V2 tests.
func setupV2Test(t *testing.T) (database.Database, *handler.Handler, http.Handler) {
	t.Helper()

	db, err := database.NewSQLiteDB("file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	tmpDir, err := os.MkdirTemp("", "dt-images-v2-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	store := storage.NewFileSystem(tmpDir)

	cfg := &config.Config{
		AuthToken:      testToken,
		BaseURL:        "http://localhost:8080",
		ImageAllowance: 100000,
	}

	h := &handler.Handler{
		DB:     db,
		Store:  store,
		Config: cfg,
	}

	router := setupV2TestRouter(h)
	return db, h, router
}

// v2BaseURL returns the API path prefix for V2 endpoints.
func v2BaseURL() string {
	return "/accounts/" + testAccountID + "/images/v2"
}

// v2AuthReq creates an *http.Request with the test bearer token.
func v2AuthReq(method, url string, body *bytes.Buffer) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, body)
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	return req
}

// v2DecodeEnvelope decodes the response body into an envelope.
func v2DecodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	return env
}

// createTestImage inserts a test image directly into the DB.
func createTestImage(t *testing.T, db database.Database, id, accountID, filename string, meta map[string]interface{}) {
	t.Helper()
	img := &model.Image{
		ID:        id,
		AccountID: accountID,
		Filename:  filename,
		Uploaded:  time.Now().UTC(),
		Meta:      meta,
	}
	require.NoError(t, db.CreateImage(img))
	if len(meta) > 0 {
		require.NoError(t, db.SetImageMetadata(accountID, id, meta))
	}
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestListImagesV2_Basic(t *testing.T) {
	db, _, router := setupV2Test(t)

	// Create a few images directly in the DB.
	createTestImage(t, db, uuid.New().String(), testAccountID, "a.jpg", nil)
	createTestImage(t, db, uuid.New().String(), testAccountID, "b.jpg", nil)
	createTestImage(t, db, uuid.New().String(), testAccountID, "c.jpg", nil)

	req := v2AuthReq("GET", v2BaseURL(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	env := v2DecodeEnvelope(t, w)
	assert.True(t, env.Success)

	var result struct {
		Images            []imageResult `json:"images"`
		ContinuationToken string        `json:"continuation_token"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &result))
	assert.Len(t, result.Images, 3)
}

func TestListImagesV2_ContinuationToken(t *testing.T) {
	db, _, router := setupV2Test(t)

	// Create 5 images with slightly staggered upload times.
	for i := 0; i < 5; i++ {
		img := &model.Image{
			ID:        uuid.New().String(),
			AccountID: testAccountID,
			Filename:  fmt.Sprintf("img%d.jpg", i),
			Uploaded:  time.Now().UTC().Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, db.CreateImage(img))
	}

	// First page: per_page=2
	req := v2AuthReq("GET", v2BaseURL()+"?per_page=2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	env := v2DecodeEnvelope(t, w)
	assert.True(t, env.Success)

	var page1 struct {
		Images            []imageResult `json:"images"`
		ContinuationToken string        `json:"continuation_token"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &page1))
	assert.Len(t, page1.Images, 2)
	assert.NotEmpty(t, page1.ContinuationToken, "should have a continuation token")

	// Second page using continuation token.
	req = v2AuthReq("GET", v2BaseURL()+"?per_page=2&continuation_token="+page1.ContinuationToken, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	env = v2DecodeEnvelope(t, w)
	assert.True(t, env.Success)

	var page2 struct {
		Images            []imageResult `json:"images"`
		ContinuationToken string        `json:"continuation_token"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &page2))
	assert.Len(t, page2.Images, 2)
	assert.NotEmpty(t, page2.ContinuationToken)

	// Third page: should have 1 image and empty token.
	req = v2AuthReq("GET", v2BaseURL()+"?per_page=2&continuation_token="+page2.ContinuationToken, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	env = v2DecodeEnvelope(t, w)

	var page3 struct {
		Images            []imageResult `json:"images"`
		ContinuationToken string        `json:"continuation_token"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &page3))
	assert.Len(t, page3.Images, 1)
	assert.Empty(t, page3.ContinuationToken, "last page should have empty token")

	// Verify no overlap between pages.
	allIDs := make(map[string]bool)
	for _, img := range page1.Images {
		allIDs[img.ID] = true
	}
	for _, img := range page2.Images {
		assert.False(t, allIDs[img.ID], "page2 image should not overlap with page1")
		allIDs[img.ID] = true
	}
	for _, img := range page3.Images {
		assert.False(t, allIDs[img.ID], "page3 image should not overlap with previous pages")
		allIDs[img.ID] = true
	}
	assert.Equal(t, 5, len(allIDs), "should have seen all 5 images across pages")
}

func TestListImagesV2_MetadataFilter(t *testing.T) {
	db, _, router := setupV2Test(t)

	// Create images with different metadata.
	createTestImage(t, db, uuid.New().String(), testAccountID, "tagged.jpg",
		map[string]interface{}{"env": "production"})
	createTestImage(t, db, uuid.New().String(), testAccountID, "tagged2.jpg",
		map[string]interface{}{"env": "staging"})
	createTestImage(t, db, uuid.New().String(), testAccountID, "no-tag.jpg", nil)

	// Filter for env=production.
	req := v2AuthReq("GET", v2BaseURL()+"?metadata[env][eq]=production", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	env := v2DecodeEnvelope(t, w)
	assert.True(t, env.Success)

	var result struct {
		Images            []imageResult `json:"images"`
		ContinuationToken string        `json:"continuation_token"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &result))
	assert.Len(t, result.Images, 1)
	assert.Equal(t, "tagged.jpg", result.Images[0].Filename)

	// Filter for env!=production should return the staging one.
	req = v2AuthReq("GET", v2BaseURL()+"?metadata[env][ne]=production", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	env = v2DecodeEnvelope(t, w)

	require.NoError(t, json.Unmarshal(env.Result, &result))
	assert.Len(t, result.Images, 1)
	assert.Equal(t, "tagged2.jpg", result.Images[0].Filename)
}

func TestCreateDirectUpload(t *testing.T) {
	_, _, router := setupV2Test(t)

	body := bytes.NewBufferString(`{}`)
	req := v2AuthReq("POST", v2BaseURL()+"/direct_upload", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	env := v2DecodeEnvelope(t, w)
	assert.True(t, env.Success)

	var result struct {
		ID        string `json:"id"`
		UploadURL string `json:"uploadURL"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &result))
	assert.NotEmpty(t, result.ID)
	assert.Contains(t, result.UploadURL, "/upload/"+result.ID)
	assert.True(t, strings.HasPrefix(result.UploadURL, "http://localhost:8080/upload/"))

	// Verify result_info is present and null in the raw response.
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Contains(t, raw, "result_info")
	assert.Nil(t, raw["result_info"])
}

func TestDirectUpload_FullFlow(t *testing.T) {
	db, _, router := setupV2Test(t)

	// Step 1: Create a direct upload.
	body := bytes.NewBufferString(`{"metadata":{"source":"direct"}}`)
	req := v2AuthReq("POST", v2BaseURL()+"/direct_upload", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	env := v2DecodeEnvelope(t, w)
	require.True(t, env.Success)

	var createResult struct {
		ID        string `json:"id"`
		UploadURL string `json:"uploadURL"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &createResult))

	// Step 2: Upload a file to the direct upload URL.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "direct-upload.png")
	require.NoError(t, err)
	_, err = fw.Write([]byte("fake-image-data"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	uploadReq := httptest.NewRequest("POST", "/upload/"+createResult.ID, &buf)
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())
	w = httptest.NewRecorder()
	router.ServeHTTP(w, uploadReq)

	require.Equal(t, http.StatusOK, w.Code)
	env = v2DecodeEnvelope(t, w)
	require.True(t, env.Success)

	var uploadResult imageResult
	require.NoError(t, json.Unmarshal(env.Result, &uploadResult))
	assert.Equal(t, createResult.ID, uploadResult.ID)
	assert.Equal(t, "direct-upload.png", uploadResult.Filename)
	assert.True(t, uploadResult.Draft, "direct upload images should start as draft")

	// Step 3: Verify the image appears in the V2 list.
	req = v2AuthReq("GET", v2BaseURL(), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	env = v2DecodeEnvelope(t, w)

	var listResult struct {
		Images []imageResult `json:"images"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &listResult))
	assert.Len(t, listResult.Images, 1)
	assert.Equal(t, createResult.ID, listResult.Images[0].ID)

	// Step 4: Verify the direct upload is marked completed (uploading again should fail).
	var buf2 bytes.Buffer
	mw2 := multipart.NewWriter(&buf2)
	fw2, err := mw2.CreateFormFile("file", "another.png")
	require.NoError(t, err)
	_, err = fw2.Write([]byte("more-data"))
	require.NoError(t, err)
	require.NoError(t, mw2.Close())

	uploadReq2 := httptest.NewRequest("POST", "/upload/"+createResult.ID, &buf2)
	uploadReq2.Header.Set("Content-Type", mw2.FormDataContentType())
	w = httptest.NewRecorder()
	router.ServeHTTP(w, uploadReq2)

	assert.Equal(t, http.StatusConflict, w.Code)

	// Step 5: Verify metadata was stored by checking it exists in DB.
	du, err := db.GetDirectUpload(createResult.ID)
	require.NoError(t, err)
	assert.True(t, du.Completed)
}

func TestDirectUpload_Expired(t *testing.T) {
	_, _, router := setupV2Test(t)

	// Create a direct upload with an expiry in the past.
	pastExpiry := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	body := bytes.NewBufferString(fmt.Sprintf(`{"expiry":"%s"}`, pastExpiry))
	req := v2AuthReq("POST", v2BaseURL()+"/direct_upload", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	env := v2DecodeEnvelope(t, w)
	require.True(t, env.Success)

	var createResult struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &createResult))

	// Attempt to upload to the expired URL.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "expired.png")
	require.NoError(t, err)
	_, err = fw.Write([]byte("fake-data"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	uploadReq := httptest.NewRequest("POST", "/upload/"+createResult.ID, &buf)
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())
	w = httptest.NewRecorder()
	router.ServeHTTP(w, uploadReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify the error message.
	var errEnv struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errEnv))
	assert.False(t, errEnv.Success)
	require.Len(t, errEnv.Errors, 1)
	assert.Equal(t, "upload URL has expired", errEnv.Errors[0].Message)
}
