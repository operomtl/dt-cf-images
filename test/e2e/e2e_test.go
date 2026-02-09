//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leca/dt-cloudflare-images/internal/config"
	"github.com/leca/dt-cloudflare-images/internal/database"
	"github.com/leca/dt-cloudflare-images/internal/router"
	"github.com/leca/dt-cloudflare-images/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testToken     = "test-token"
	testAccountID = "test-account"
)

// setupTestServer creates a test HTTP server backed by in-memory SQLite
// and a temporary filesystem storage directory.
func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	db, err := database.NewSQLiteDB("file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	store := storage.NewFileSystem(t.TempDir())
	cfg := &config.Config{
		AuthToken:      testToken,
		BaseURL:        "", // will be set after server starts
		ImageAllowance: 100000,
	}
	srv := router.New(db, store, cfg)
	ts := httptest.NewServer(srv.Router)
	cfg.BaseURL = ts.URL
	t.Cleanup(ts.Close)
	return ts
}

// apiBase returns the base URL for the images API.
func apiBase(ts *httptest.Server) string {
	return ts.URL + "/accounts/" + testAccountID + "/images"
}

// makeJPEG creates a small valid JPEG image in memory and returns the bytes.
func makeJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// createUploadRequest builds a multipart POST request to upload a JPEG file.
func createUploadRequest(t *testing.T, url, token string) *http.Request {
	t.Helper()

	imgBytes := makeJPEG(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.jpg")
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(imgBytes))
	require.NoError(t, err)
	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// createUploadRequestWithMetadata builds a multipart POST with file + metadata.
func createUploadRequestWithMetadata(t *testing.T, url, token string, metadata map[string]interface{}) *http.Request {
	t.Helper()

	imgBytes := makeJPEG(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.jpg")
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(imgBytes))
	require.NoError(t, err)

	if metadata != nil {
		metaJSON, err := json.Marshal(metadata)
		require.NoError(t, err)
		require.NoError(t, writer.WriteField("metadata", string(metaJSON)))
	}

	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// envelope is a generic Cloudflare API response envelope.
type envelope struct {
	Success bool            `json:"success"`
	Errors  []interface{}   `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

// paginatedEnvelope adds result_info for paginated responses.
type paginatedEnvelope struct {
	Success    bool            `json:"success"`
	Errors     []interface{}   `json:"errors"`
	Result     json.RawMessage `json:"result"`
	ResultInfo struct {
		Page       int `json:"page"`
		PerPage    int `json:"per_page"`
		Count      int `json:"count"`
		TotalCount int `json:"total_count"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

// imageResult represents the fields we check in image responses.
type imageResult struct {
	ID                string                 `json:"id"`
	Filename          string                 `json:"filename"`
	Meta              map[string]interface{} `json:"meta"`
	RequireSignedURLs bool                   `json:"requireSignedURLs"`
	Uploaded          string                 `json:"uploaded"`
	Variants          []string               `json:"variants"`
}

// doRequest performs an HTTP request and returns the response.
func doRequest(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// decodeEnvelope reads the body and decodes it as an envelope.
func decodeEnvelope(t *testing.T, resp *http.Response) envelope {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var env envelope
	require.NoError(t, json.Unmarshal(data, &env))
	return env
}

// authGet performs an authenticated GET request.
func authGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testToken)
	return doRequest(t, req)
}

// authDelete performs an authenticated DELETE request.
func authDelete(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testToken)
	return doRequest(t, req)
}

// authPatch performs an authenticated PATCH request with a JSON body.
func authPatch(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	return doRequest(t, req)
}

// authPost performs an authenticated POST request with a JSON body.
func authPost(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	return doRequest(t, req)
}

// authPut performs an authenticated PUT request.
func authPut(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest("PUT", url, reader)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	return doRequest(t, req)
}

// uploadImage uploads a JPEG image and returns the decoded image result.
func uploadImage(t *testing.T, ts *httptest.Server) imageResult {
	t.Helper()
	req := createUploadRequest(t, apiBase(ts)+"/v1", testToken)
	resp := doRequest(t, req)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env := decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var img imageResult
	require.NoError(t, json.Unmarshal(env.Result, &img))
	return img
}

// ---------------------------------------------------------------------------
// Test Cases
// ---------------------------------------------------------------------------

func TestImageLifecycle(t *testing.T) {
	ts := setupTestServer(t)

	jpegBytes := makeJPEG(t)

	// 1. Upload image via multipart POST /v1
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "lifecycle.jpg")
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(jpegBytes))
	require.NoError(t, err)
	writer.Close()

	req, err := http.NewRequest("POST", apiBase(ts)+"/v1", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp := doRequest(t, req)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env := decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var uploaded imageResult
	require.NoError(t, json.Unmarshal(env.Result, &uploaded))

	// 2. Verify response has id, filename, uploaded time
	assert.NotEmpty(t, uploaded.ID, "image should have an ID")
	assert.Equal(t, "lifecycle.jpg", uploaded.Filename)
	assert.NotEmpty(t, uploaded.Uploaded, "image should have an uploaded timestamp")

	imageID := uploaded.ID

	// 3. GET /v1/{id} - verify matches
	resp = authGet(t, apiBase(ts)+"/v1/"+imageID)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var fetched imageResult
	require.NoError(t, json.Unmarshal(env.Result, &fetched))
	assert.Equal(t, imageID, fetched.ID)
	assert.Equal(t, "lifecycle.jpg", fetched.Filename)

	// 4. PATCH /v1/{id} - update metadata
	patchBody := map[string]interface{}{
		"metadata": map[string]interface{}{"env": "test"},
	}
	resp = authPatch(t, apiBase(ts)+"/v1/"+imageID, patchBody)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	// 5. GET /v1/{id} - verify metadata updated
	resp = authGet(t, apiBase(ts)+"/v1/"+imageID)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var updated imageResult
	require.NoError(t, json.Unmarshal(env.Result, &updated))
	assert.Equal(t, "test", updated.Meta["env"])

	// 6. GET /v1/{id}/blob - verify content matches uploaded bytes
	resp = authGet(t, apiBase(ts)+"/v1/"+imageID+"/blob")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	blobData, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, jpegBytes, blobData, "blob content should match uploaded bytes")

	// 7. GET /v1 - verify image appears in list
	resp = authGet(t, apiBase(ts)+"/v1")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()
	listData, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var listEnv paginatedEnvelope
	require.NoError(t, json.Unmarshal(listData, &listEnv))
	require.True(t, listEnv.Success)
	assert.GreaterOrEqual(t, listEnv.ResultInfo.TotalCount, 1)

	var listResult struct {
		Images []imageResult `json:"images"`
	}
	require.NoError(t, json.Unmarshal(listEnv.Result, &listResult))
	found := false
	for _, img := range listResult.Images {
		if img.ID == imageID {
			found = true
			break
		}
	}
	assert.True(t, found, "uploaded image should appear in list")

	// 8. DELETE /v1/{id} - verify 200
	resp = authDelete(t, apiBase(ts)+"/v1/"+imageID)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	// 9. GET /v1/{id} - verify 404
	resp = authGet(t, apiBase(ts)+"/v1/"+imageID)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestVariantLifecycle(t *testing.T) {
	ts := setupTestServer(t)

	// 1. Create variant POST /v1/variants
	createBody := map[string]interface{}{
		"id": "thumbnail",
		"options": map[string]interface{}{
			"fit":      "scale-down",
			"width":    150,
			"height":   150,
			"metadata": "none",
		},
	}
	resp := authPost(t, apiBase(ts)+"/v1/variants", createBody)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env := decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var created struct {
		ID      string `json:"id"`
		Options struct {
			Fit      string `json:"fit"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
			Metadata string `json:"metadata"`
		} `json:"options"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &created))
	assert.Equal(t, "thumbnail", created.ID)
	assert.Equal(t, "scale-down", created.Options.Fit)
	assert.Equal(t, 150, created.Options.Width)
	assert.Equal(t, 150, created.Options.Height)

	// 2. List variants GET /v1/variants - verify it appears
	resp = authGet(t, apiBase(ts)+"/v1/variants")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var listResult struct {
		Variants map[string]json.RawMessage `json:"variants"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &listResult))
	assert.Contains(t, listResult.Variants, "thumbnail", "variant should appear in list")

	// 3. Get variant GET /v1/variants/thumbnail
	resp = authGet(t, apiBase(ts)+"/v1/variants/thumbnail")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var gotVariant struct {
		ID      string `json:"id"`
		Options struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"options"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &gotVariant))
	assert.Equal(t, "thumbnail", gotVariant.ID)
	assert.Equal(t, 150, gotVariant.Options.Width)

	// 4. Update variant PATCH /v1/variants/thumbnail - change width to 200
	updateBody := map[string]interface{}{
		"options": map[string]interface{}{
			"width": 200,
		},
	}
	resp = authPatch(t, apiBase(ts)+"/v1/variants/thumbnail", updateBody)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	// 5. Verify update persisted
	resp = authGet(t, apiBase(ts)+"/v1/variants/thumbnail")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var updatedVariant struct {
		Options struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"options"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &updatedVariant))
	assert.Equal(t, 200, updatedVariant.Options.Width, "width should be updated to 200")
	assert.Equal(t, 150, updatedVariant.Options.Height, "height should remain 150")

	// 6. Delete variant DELETE /v1/variants/thumbnail
	resp = authDelete(t, apiBase(ts)+"/v1/variants/thumbnail")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	// 7. Verify GET returns 404
	resp = authGet(t, apiBase(ts)+"/v1/variants/thumbnail")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestSigningKeyLifecycle(t *testing.T) {
	ts := setupTestServer(t)

	// 1. PUT /v1/keys/mykey - create key
	resp := authPut(t, apiBase(ts)+"/v1/keys/mykey", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env := decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var createdKey struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &createdKey))
	assert.Equal(t, "mykey", createdKey.Name)
	assert.NotEmpty(t, createdKey.Value, "key should have a generated value")

	// 2. GET /v1/keys - verify it appears with value
	resp = authGet(t, apiBase(ts)+"/v1/keys")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var keysResult struct {
		Keys []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &keysResult))
	require.Len(t, keysResult.Keys, 1)
	assert.Equal(t, "mykey", keysResult.Keys[0].Name)
	assert.NotEmpty(t, keysResult.Keys[0].Value)

	// 3. DELETE /v1/keys/mykey - delete it
	resp = authDelete(t, apiBase(ts)+"/v1/keys/mykey")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	// 4. GET /v1/keys - verify a "default" key was auto-created
	resp = authGet(t, apiBase(ts)+"/v1/keys")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	require.NoError(t, json.Unmarshal(env.Result, &keysResult))
	require.Len(t, keysResult.Keys, 1, "should have auto-created default key")
	assert.Equal(t, "default", keysResult.Keys[0].Name)
	assert.NotEmpty(t, keysResult.Keys[0].Value)
}

func TestDirectUploadFlow(t *testing.T) {
	ts := setupTestServer(t)
	jpegBytes := makeJPEG(t)

	// 1. POST /v2/direct_upload - get upload URL
	resp := authPost(t, apiBase(ts)+"/v2/direct_upload", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env := decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var duResult struct {
		ID        string `json:"id"`
		UploadURL string `json:"uploadURL"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &duResult))
	assert.NotEmpty(t, duResult.ID)
	assert.NotEmpty(t, duResult.UploadURL)

	// 2. POST to upload URL with file (no auth needed)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "direct.jpg")
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(jpegBytes))
	require.NoError(t, err)
	writer.Close()

	uploadReq, err := http.NewRequest("POST", duResult.UploadURL, body)
	require.NoError(t, err)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	// Note: no Authorization header needed for direct upload

	resp = doRequest(t, uploadReq)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var uploadedImg imageResult
	require.NoError(t, json.Unmarshal(env.Result, &uploadedImg))
	assert.Equal(t, duResult.ID, uploadedImg.ID, "image ID should match the direct upload ID")
	assert.Equal(t, "direct.jpg", uploadedImg.Filename)

	// 3. GET /v2 - verify image appears in V2 list
	resp = authGet(t, apiBase(ts)+"/v2")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var v2Result struct {
		Images            []imageResult `json:"images"`
		ContinuationToken string        `json:"continuation_token"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &v2Result))
	found := false
	for _, img := range v2Result.Images {
		if img.ID == duResult.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "direct-uploaded image should appear in V2 list")

	// 4. GET /v1/{id} - verify image details
	resp = authGet(t, apiBase(ts)+"/v1/"+duResult.ID)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var detail imageResult
	require.NoError(t, json.Unmarshal(env.Result, &detail))
	assert.Equal(t, duResult.ID, detail.ID)
	assert.Equal(t, "direct.jpg", detail.Filename)
}

func TestV2ListWithPagination(t *testing.T) {
	ts := setupTestServer(t)

	// 1. Upload 5 images
	uploadedIDs := make(map[string]bool)
	for i := 0; i < 5; i++ {
		img := uploadImage(t, ts)
		uploadedIDs[img.ID] = true
	}
	require.Len(t, uploadedIDs, 5, "should have 5 unique image IDs")

	// 2. GET /v2?per_page=2 - get first page with continuation_token
	allFound := make(map[string]bool)
	nextToken := ""
	pageCount := 0

	for {
		url := apiBase(ts) + "/v2?per_page=2"
		if nextToken != "" {
			url += "&continuation_token=" + nextToken
		}

		resp := authGet(t, url)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		env := decodeEnvelope(t, resp)
		require.True(t, env.Success)

		var pageResult struct {
			Images            []imageResult `json:"images"`
			ContinuationToken string        `json:"continuation_token"`
		}
		require.NoError(t, json.Unmarshal(env.Result, &pageResult))

		for _, img := range pageResult.Images {
			assert.False(t, allFound[img.ID], "image %s should not appear twice (no duplicates)", img.ID)
			allFound[img.ID] = true
		}

		pageCount++
		nextToken = pageResult.ContinuationToken

		if nextToken == "" {
			break
		}

		// Safety: don't loop forever
		require.Less(t, pageCount, 10, "pagination should complete within a reasonable number of pages")
	}

	// 5. Verify no duplicates and all 5 found
	assert.Len(t, allFound, 5, "should have found all 5 images across pages")
	for id := range uploadedIDs {
		assert.True(t, allFound[id], "image %s should have been found in pagination", id)
	}
}

func TestStatsEndpoint(t *testing.T) {
	ts := setupTestServer(t)

	// 1. GET /v1/stats - verify count is 0
	resp := authGet(t, apiBase(ts)+"/v1/stats")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env := decodeEnvelope(t, resp)
	require.True(t, env.Success)

	var stats struct {
		Count struct {
			Current float64 `json:"current"`
			Allowed float64 `json:"allowed"`
		} `json:"count"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &stats))
	assert.Equal(t, float64(0), stats.Count.Current)
	assert.Equal(t, float64(100000), stats.Count.Allowed)

	// 2. Upload 3 images
	for i := 0; i < 3; i++ {
		uploadImage(t, ts)
	}

	// 3. GET /v1/stats - verify count is 3
	resp = authGet(t, apiBase(ts)+"/v1/stats")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	env = decodeEnvelope(t, resp)
	require.True(t, env.Success)

	require.NoError(t, json.Unmarshal(env.Result, &stats))
	assert.Equal(t, float64(3), stats.Count.Current)
}

func TestAuthRequired(t *testing.T) {
	ts := setupTestServer(t)

	// 1. Try GET /v1 without auth header - verify 401
	req, err := http.NewRequest("GET", apiBase(ts)+"/v1", nil)
	require.NoError(t, err)
	resp := doRequest(t, req)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// 2. Try with wrong token - verify 401
	req, err = http.NewRequest("GET", apiBase(ts)+"/v1", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp = doRequest(t, req)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// 3. Try with correct token - verify 200
	resp = authGet(t, apiBase(ts)+"/v1")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestErrorScenarios(t *testing.T) {
	ts := setupTestServer(t)

	// 1. GET /v1/nonexistent - 404
	resp := authGet(t, apiBase(ts)+"/v1/nonexistent")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 2. DELETE /v1/nonexistent - 404
	resp = authDelete(t, apiBase(ts)+"/v1/nonexistent")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// 3. POST /v1 without file - 400
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", `{"key":"val"}`)
	writer.Close()

	req, err := http.NewRequest("POST", apiBase(ts)+"/v1", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp = doRequest(t, req)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestImageLifecycle_VariantURLsInResponse(t *testing.T) {
	// This test verifies that when variants exist, image responses include variant URLs.
	ts := setupTestServer(t)

	// Create a variant first
	createBody := map[string]interface{}{
		"id": "hero",
		"options": map[string]interface{}{
			"fit":      "cover",
			"width":    800,
			"height":   600,
			"metadata": "none",
		},
	}
	resp := authPost(t, apiBase(ts)+"/v1/variants", createBody)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Upload image
	img := uploadImage(t, ts)
	assert.NotEmpty(t, img.Variants, "image should have variant URLs when variants exist")

	// Verify the variant URL contains the expected pattern
	found := false
	expected := fmt.Sprintf("/cdn/%s/%s/hero", testAccountID, img.ID)
	for _, v := range img.Variants {
		if len(v) > len(expected) {
			// Check suffix since the URL includes the base URL
			if v[len(v)-len(expected):] == expected {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "should have a variant URL containing %s, got %v", expected, img.Variants)
}
