package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuccessResponse(t *testing.T) {
	result := map[string]string{"id": "abc-123"}
	resp := SuccessResponse(result)

	assert.True(t, resp.Success)
	assert.Equal(t, result, resp.Result)
	assert.Empty(t, resp.Errors)
	assert.Empty(t, resp.Messages)
}

func TestSuccessResponseNilResult(t *testing.T) {
	resp := SuccessResponse(nil)

	assert.True(t, resp.Success)
	assert.Nil(t, resp.Result)
	assert.Empty(t, resp.Errors)
	assert.Empty(t, resp.Messages)
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse(9400, "bad request")

	assert.False(t, resp.Success)
	assert.Nil(t, resp.Result)
	assert.Len(t, resp.Errors, 1)
	assert.Equal(t, 9400, resp.Errors[0].Code)
	assert.Equal(t, "bad request", resp.Errors[0].Message)
	assert.Empty(t, resp.Messages)
}

func TestPaginatedResponse(t *testing.T) {
	items := []string{"a", "b"}
	info := ResultInfo{
		Page:       1,
		PerPage:    20,
		Count:      2,
		TotalCount: 50,
		TotalPages: 3,
	}

	resp := PaginatedResponse(items, info)

	assert.Equal(t, true, resp["success"])
	assert.Equal(t, items, resp["result"])
	assert.Equal(t, info, resp["result_info"])
	assert.Empty(t, resp["errors"])
	assert.Empty(t, resp["messages"])
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	body := SuccessResponse(map[string]string{"hello": "world"})

	WriteJSON(w, http.StatusOK, body)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var decoded Response
	err := json.NewDecoder(res.Body).Decode(&decoded)
	require.NoError(t, err)
	assert.True(t, decoded.Success)
}

func TestWriteJSONCustomStatus(t *testing.T) {
	w := httptest.NewRecorder()
	body := ErrorResponse(9404, "not found")

	WriteJSON(w, http.StatusNotFound, body)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var decoded Response
	err := json.NewDecoder(res.Body).Decode(&decoded)
	require.NoError(t, err)
	assert.False(t, decoded.Success)
	assert.Len(t, decoded.Errors, 1)
	assert.Equal(t, "not found", decoded.Errors[0].Message)
}

func TestErrorResponseJSONStructure(t *testing.T) {
	// Verify the JSON output matches the Cloudflare API shape exactly.
	resp := ErrorResponse(9401, "Authentication required")
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusUnauthorized, resp)

	var raw map[string]interface{}
	err := json.NewDecoder(w.Result().Body).Decode(&raw)
	require.NoError(t, err)

	assert.Nil(t, raw["result"])
	assert.Equal(t, false, raw["success"])

	errors, ok := raw["errors"].([]interface{})
	require.True(t, ok)
	require.Len(t, errors, 1)

	errObj, ok := errors[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(9401), errObj["code"])
	assert.Equal(t, "Authentication required", errObj["message"])
}
