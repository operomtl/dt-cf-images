package handler

import (
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leca/dt-cloudflare-images/internal/api"
)

// GetImageBlob handles GET /v1/{image_id}/blob -- streams the original image bytes.
func (h *Handler) GetImageBlob(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	imageID := chi.URLParam(r, "image_id")

	// Verify image record exists.
	img, err := h.DB.GetImage(accountID, imageID)
	if err != nil || img == nil {
		api.NotFound(w, "image not found")
		return
	}

	rc, err := h.Store.Retrieve(accountID, imageID)
	if err != nil {
		api.NotFound(w, "image blob not found")
		return
	}
	defer rc.Close()

	// Read up to 512 bytes for content-type detection.
	buf := make([]byte, 512)
	n, err := io.ReadAtLeast(rc, buf, 1)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to read image"))
		return
	}
	buf = buf[:n]

	contentType := http.DetectContentType(buf)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+img.Filename+"\"")
	w.WriteHeader(http.StatusOK)

	// Write the already-read bytes first, then stream the rest.
	if _, err := w.Write(buf); err != nil {
		log.Printf("GetImageBlob: failed to write response: %v", err)
		return
	}
	if _, err := io.Copy(w, rc); err != nil {
		log.Printf("GetImageBlob: failed to stream response: %v", err)
	}
}
