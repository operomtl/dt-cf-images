package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leca/dt-cloudflare-images/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testJPEG generates a small valid JPEG image.
func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// testPNG generates a small valid PNG image.
func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// signURL computes an HMAC-SHA256 signature for the given path and expiry.
func signURL(keyValue, path, expStr string) string {
	mac := hmac.New(sha256.New, []byte(keyValue))
	mac.Write([]byte(path + expStr))
	return hex.EncodeToString(mac.Sum(nil))
}

func setupDeliverRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/cdn/{account_id}/{image_id}/{variant_name}", h.DeliverImage)
	return r
}

// seedImageAndVariant creates a DB image record, a variant, and stores image bytes.
func seedImageAndVariant(t *testing.T, h *Handler, imageID, variantName string, imgData []byte, requireSigned bool, neverRequireSigned bool) {
	t.Helper()
	img := &model.Image{
		ID:                imageID,
		AccountID:         testAccountID,
		Filename:          "test.jpg",
		RequireSignedURLs: requireSigned,
		Uploaded:          time.Now().UTC(),
	}
	require.NoError(t, h.DB.CreateImage(img))

	variant := &model.Variant{
		ID:                     variantName,
		AccountID:              testAccountID,
		Options:                model.VariantOptions{Fit: "scale-down", Width: 100, Height: 100, Metadata: "none"},
		NeverRequireSignedURLs: neverRequireSigned,
	}
	require.NoError(t, h.DB.CreateVariant(variant))

	_, err := h.Store.Store(testAccountID, imageID, bytes.NewReader(imgData))
	require.NoError(t, err)
}

func TestDeliverImage_SuccessJPEG(t *testing.T) {
	h := newTestHandler(t)
	router := setupDeliverRouter(h)

	data := testJPEG(t)
	seedImageAndVariant(t, h, "img-1", "thumb", data, false, false)

	req := httptest.NewRequest(http.MethodGet, "/cdn/"+testAccountID+"/img-1/thumb", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Body.Bytes())
}

func TestDeliverImage_SuccessPNG(t *testing.T) {
	h := newTestHandler(t)
	router := setupDeliverRouter(h)

	data := testPNG(t)
	seedImageAndVariant(t, h, "img-2", "thumb", data, false, false)

	req := httptest.NewRequest(http.MethodGet, "/cdn/"+testAccountID+"/img-2/thumb", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Body.Bytes())
}

func TestDeliverImage_ImageNotFound(t *testing.T) {
	h := newTestHandler(t)
	router := setupDeliverRouter(h)

	// Create variant but no image.
	variant := &model.Variant{
		ID:        "thumb",
		AccountID: testAccountID,
		Options:   model.VariantOptions{Fit: "scale-down", Width: 100, Height: 100, Metadata: "none"},
	}
	require.NoError(t, h.DB.CreateVariant(variant))

	req := httptest.NewRequest(http.MethodGet, "/cdn/"+testAccountID+"/nonexistent/thumb", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeliverImage_VariantNotFound(t *testing.T) {
	h := newTestHandler(t)
	router := setupDeliverRouter(h)

	data := testJPEG(t)
	img := &model.Image{
		ID:        "img-3",
		AccountID: testAccountID,
		Filename:  "test.jpg",
		Uploaded:  time.Now().UTC(),
	}
	require.NoError(t, h.DB.CreateImage(img))
	_, err := h.Store.Store(testAccountID, "img-3", bytes.NewReader(data))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/cdn/"+testAccountID+"/img-3/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeliverImage_NoBlobInStorage(t *testing.T) {
	h := newTestHandler(t)
	router := setupDeliverRouter(h)

	// Create image and variant DB records but don't store any bytes.
	img := &model.Image{
		ID:        "img-4",
		AccountID: testAccountID,
		Filename:  "test.jpg",
		Uploaded:  time.Now().UTC(),
	}
	require.NoError(t, h.DB.CreateImage(img))
	variant := &model.Variant{
		ID:        "thumb",
		AccountID: testAccountID,
		Options:   model.VariantOptions{Fit: "scale-down", Width: 100, Height: 100, Metadata: "none"},
	}
	require.NoError(t, h.DB.CreateVariant(variant))

	req := httptest.NewRequest(http.MethodGet, "/cdn/"+testAccountID+"/img-4/thumb", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeliverImage_SignedURL_Valid(t *testing.T) {
	h := newTestHandler(t)
	h.Config.EnforceSignedURLs = true
	router := setupDeliverRouter(h)

	data := testJPEG(t)
	seedImageAndVariant(t, h, "img-5", "thumb", data, true, false)

	// Create a signing key.
	key := &model.SigningKey{
		Name:      "key1",
		Value:     "test-secret-key-value",
		AccountID: testAccountID,
	}
	require.NoError(t, h.DB.CreateSigningKey(key))

	exp := time.Now().Add(time.Hour).Unix()
	expStr := strconv.FormatInt(exp, 10)
	path := fmt.Sprintf("/cdn/%s/img-5/thumb", testAccountID)
	sig := signURL(key.Value, path, expStr)

	url := fmt.Sprintf("%s?sig=%s&exp=%s", path, sig, expStr)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
}

func TestDeliverImage_SignedURL_Expired(t *testing.T) {
	h := newTestHandler(t)
	h.Config.EnforceSignedURLs = true
	router := setupDeliverRouter(h)

	data := testJPEG(t)
	seedImageAndVariant(t, h, "img-6", "thumb", data, true, false)

	key := &model.SigningKey{
		Name:      "key1",
		Value:     "test-secret-key-value",
		AccountID: testAccountID,
	}
	require.NoError(t, h.DB.CreateSigningKey(key))

	// Expired 1 hour ago.
	exp := time.Now().Add(-time.Hour).Unix()
	expStr := strconv.FormatInt(exp, 10)
	path := fmt.Sprintf("/cdn/%s/img-6/thumb", testAccountID)
	sig := signURL(key.Value, path, expStr)

	url := fmt.Sprintf("%s?sig=%s&exp=%s", path, sig, expStr)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeliverImage_SignedURL_WrongSignature(t *testing.T) {
	h := newTestHandler(t)
	h.Config.EnforceSignedURLs = true
	router := setupDeliverRouter(h)

	data := testJPEG(t)
	seedImageAndVariant(t, h, "img-7", "thumb", data, true, false)

	key := &model.SigningKey{
		Name:      "key1",
		Value:     "test-secret-key-value",
		AccountID: testAccountID,
	}
	require.NoError(t, h.DB.CreateSigningKey(key))

	exp := time.Now().Add(time.Hour).Unix()
	expStr := strconv.FormatInt(exp, 10)
	path := fmt.Sprintf("/cdn/%s/img-7/thumb", testAccountID)

	url := fmt.Sprintf("%s?sig=%s&exp=%s", path, "deadbeef", expStr)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeliverImage_SignedURL_MissingParams(t *testing.T) {
	h := newTestHandler(t)
	h.Config.EnforceSignedURLs = true
	router := setupDeliverRouter(h)

	data := testJPEG(t)
	seedImageAndVariant(t, h, "img-8", "thumb", data, true, false)

	key := &model.SigningKey{
		Name:      "key1",
		Value:     "test-secret-key-value",
		AccountID: testAccountID,
	}
	require.NoError(t, h.DB.CreateSigningKey(key))

	// No sig or exp params.
	path := fmt.Sprintf("/cdn/%s/img-8/thumb", testAccountID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeliverImage_SignedURL_NeverRequireSignedURLs(t *testing.T) {
	h := newTestHandler(t)
	h.Config.EnforceSignedURLs = true
	router := setupDeliverRouter(h)

	data := testJPEG(t)
	// Image requires signed URLs, but variant has neverRequireSignedURLs=true.
	seedImageAndVariant(t, h, "img-9", "public-thumb", data, true, true)

	// No sig/exp — should still work because variant bypasses.
	path := fmt.Sprintf("/cdn/%s/img-9/public-thumb", testAccountID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
}

func TestDeliverImage_SignedURL_EnforcementOff(t *testing.T) {
	h := newTestHandler(t)
	// EnforceSignedURLs is false (default).
	router := setupDeliverRouter(h)

	data := testJPEG(t)
	// Image requires signed URLs, but enforcement is off.
	seedImageAndVariant(t, h, "img-10", "thumb", data, true, false)

	path := fmt.Sprintf("/cdn/%s/img-10/thumb", testAccountID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
}
