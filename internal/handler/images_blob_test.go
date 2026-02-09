package handler_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetImageBlob(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	content := []byte("fake-png-image-blob-content-for-testing")
	uploaded := uploadAndDecode(t, ts, content, "photo.png")

	req := authReq("GET", baseURL(ts)+"/"+uploaded.ID+"/blob", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, content, body)

	// Content-Disposition should reference the original filename.
	cd := resp.Header.Get("Content-Disposition")
	assert.Contains(t, cd, "photo.png")
}

func TestGetImageBlob_NotFound(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()

	req := authReq("GET", baseURL(ts)+"/nonexistent-id/blob", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
