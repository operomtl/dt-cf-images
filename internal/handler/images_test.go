package handler_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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

// testServer creates a test HTTP server backed by in-memory SQLite
// and a temporary filesystem storage directory.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()

	db, err := database.NewSQLiteDB("file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	tmpDir, err := os.MkdirTemp("", "dt-images-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	store := storage.NewFileSystem(tmpDir)

	cfg := &config.Config{
		AuthToken:      testToken,
		BaseURL:        "http://localhost:8080",
		ImageAllowance: 100000,
	}

	srv := router.New(db, store, cfg)
	return httptest.NewServer(srv.Router)
}

// baseURL returns the API path prefix for our test account.
func baseURL(ts *httptest.Server) string {
	return ts.URL + "/accounts/" + testAccountID + "/images/v1"
}

// authReq creates an *http.Request with the test bearer token.
func authReq(method, url string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer "+testToken)
	return req
}

// multipartFileBody builds a multipart request body with a file field.
func multipartFileBody(t *testing.T, fieldName, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

// decodeResponse decodes the JSON body into the provided target.
func decodeResponse(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, target))
}

// envelope is a generic Cloudflare API envelope for assertions.
type envelope struct {
	Success bool            `json:"success"`
	Errors  []interface{}   `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

// paginatedEnvelope adds result_info.
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
	Variants          []string               `json:"variants"`
	Draft             bool                   `json:"draft"`
}

// uploadFile is a helper that uploads a file and returns the response.
func uploadFile(t *testing.T, ts *httptest.Server, content []byte, fileName string) *http.Response {
	t.Helper()
	body, contentType := multipartFileBody(t, "file", fileName, content)
	req := authReq("POST", baseURL(ts), body)
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// uploadAndDecode uploads a file and decodes the response into an imageResult.
func uploadAndDecode(t *testing.T, ts *httptest.Server, content []byte, fileName string) imageResult {
	t.Helper()
	resp := uploadFile(t, ts, content, fileName)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var env envelope
	decodeResponse(t, resp, &env)
	assert.True(t, env.Success)
	var img imageResult
	require.NoError(t, json.Unmarshal(env.Result, &img))
	return img
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestUploadImage_File(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	content := []byte("fake-image-data")
	img := uploadAndDecode(t, ts, content, "photo.png")

	assert.NotEmpty(t, img.ID)
	assert.Equal(t, "photo.png", img.Filename)
}

func TestUploadImage_MissingFile(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	// Send a multipart form with no file or url field.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("metadata", `{"key":"val"}`))
	w.Close()

	req := authReq("POST", baseURL(ts), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetImage(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	uploaded := uploadAndDecode(t, ts, []byte("data"), "test.jpg")

	req := authReq("GET", baseURL(ts)+"/"+uploaded.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	var env envelope
	decodeResponse(t, resp, &env)
	assert.True(t, env.Success)

	var img imageResult
	require.NoError(t, json.Unmarshal(env.Result, &img))
	assert.Equal(t, uploaded.ID, img.ID)
	assert.Equal(t, "test.jpg", img.Filename)
}

func TestGetImage_NotFound(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	req := authReq("GET", baseURL(ts)+"/nonexistent-id", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListImages(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	// Upload a few images.
	uploadAndDecode(t, ts, []byte("img1"), "a.png")
	uploadAndDecode(t, ts, []byte("img2"), "b.png")
	uploadAndDecode(t, ts, []byte("img3"), "c.png")

	req := authReq("GET", baseURL(ts), nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	var env paginatedEnvelope
	decodeResponse(t, resp, &env)
	assert.True(t, env.Success)
	assert.Equal(t, 3, env.ResultInfo.TotalCount)
	assert.Equal(t, 3, env.ResultInfo.Count)

	var listResult struct {
		Images []imageResult `json:"images"`
	}
	require.NoError(t, json.Unmarshal(env.Result, &listResult))
	assert.Len(t, listResult.Images, 3)
}

func TestListImages_Pagination(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	// Upload 5 images.
	for i := 0; i < 5; i++ {
		uploadAndDecode(t, ts, []byte("img"), "img.png")
	}

	// Request page 1 with per_page=2.
	req := authReq("GET", baseURL(ts)+"?page=1&per_page=2", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	var env paginatedEnvelope
	decodeResponse(t, resp, &env)
	assert.True(t, env.Success)
	assert.Equal(t, 1, env.ResultInfo.Page)
	assert.Equal(t, 2, env.ResultInfo.PerPage)
	assert.Equal(t, 2, env.ResultInfo.Count)
	assert.Equal(t, 5, env.ResultInfo.TotalCount)
	assert.Equal(t, 3, env.ResultInfo.TotalPages)

	// Request page 3 -- should have 1 item.
	req = authReq("GET", baseURL(ts)+"?page=3&per_page=2", nil)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)

	decodeResponse(t, resp, &env)
	assert.Equal(t, 1, env.ResultInfo.Count)
}

func TestUpdateImage(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	uploaded := uploadAndDecode(t, ts, []byte("data"), "original.png")

	updateBody := `{"metadata":{"key":"value"},"requireSignedURLs":true}`
	req := authReq("PATCH", baseURL(ts)+"/"+uploaded.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	var env envelope
	decodeResponse(t, resp, &env)
	assert.True(t, env.Success)

	var img imageResult
	require.NoError(t, json.Unmarshal(env.Result, &img))
	assert.True(t, img.RequireSignedURLs)
	assert.Equal(t, "value", img.Meta["key"])
}

func TestDeleteImage(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	uploaded := uploadAndDecode(t, ts, []byte("data"), "to-delete.png")

	req := authReq("DELETE", baseURL(ts)+"/"+uploaded.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	var env envelope
	decodeResponse(t, resp, &env)
	assert.True(t, env.Success)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify it is gone.
	req = authReq("GET", baseURL(ts)+"/"+uploaded.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteImage_NotFound(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	req := authReq("DELETE", baseURL(ts)+"/nonexistent-id", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
