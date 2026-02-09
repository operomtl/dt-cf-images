package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leca/dt-cloudflare-images/internal/imageproc"
)

// DeliverImage handles GET /cdn/{account_id}/{image_id}/{variant_name} --
// serves a transformed image, optionally enforcing signed URLs.
func (h *Handler) DeliverImage(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "account_id")
	imageID := chi.URLParam(r, "image_id")
	variantName := chi.URLParam(r, "variant_name")

	img, err := h.DB.GetImage(accountID, imageID)
	if err != nil || img == nil {
		http.Error(w, "image not found", http.StatusNotFound)
		return
	}

	variant, err := h.DB.GetVariant(accountID, variantName)
	if err != nil || variant == nil {
		http.Error(w, "variant not found", http.StatusNotFound)
		return
	}

	// Signed URL enforcement.
	if h.Config.EnforceSignedURLs && img.RequireSignedURLs && !variant.NeverRequireSignedURLs {
		if !h.verifySignature(r, accountID, imageID, variantName) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	rc, err := h.Store.Retrieve(accountID, imageID)
	if err != nil {
		http.Error(w, "image not found", http.StatusNotFound)
		return
	}
	defer rc.Close()

	transformed, format, err := imageproc.Transform(rc, variant.Options)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	ct := formatToContentType(format)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", strconv.Itoa(len(transformed)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(transformed); err != nil {
		log.Printf("DeliverImage: failed to write response: %v", err)
	}
}

// verifySignature checks the sig and exp query parameters against the
// account's signing keys. Returns true if the signature is valid and not expired.
func (h *Handler) verifySignature(r *http.Request, accountID, imageID, variantName string) bool {
	sigHex := r.URL.Query().Get("sig")
	expStr := r.URL.Query().Get("exp")
	if sigHex == "" || expStr == "" {
		return false
	}

	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expUnix {
		return false
	}

	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	keys, err := h.DB.ListSigningKeys(accountID)
	if err != nil || len(keys) == 0 {
		return false
	}

	path := fmt.Sprintf("/cdn/%s/%s/%s", accountID, imageID, variantName)
	message := path + expStr

	for _, key := range keys {
		mac := hmac.New(sha256.New, []byte(key.Value))
		mac.Write([]byte(message))
		expected := mac.Sum(nil)
		if hmac.Equal(sig, expected) {
			return true
		}
	}
	return false
}

// formatToContentType maps an image format string to its MIME type.
func formatToContentType(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}
