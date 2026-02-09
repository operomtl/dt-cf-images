package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/leca/dt-cloudflare-images/internal/api"
	"github.com/leca/dt-cloudflare-images/internal/model"
)

// buildVariantURLs constructs the variant URL list for an image by
// querying all variants defined for the account.
func (h *Handler) buildVariantURLs(accountID, imageID string) []string {
	variants, err := h.DB.ListVariants(accountID)
	if err != nil || len(variants) == 0 {
		return []string{}
	}
	urls := make([]string, 0, len(variants))
	base := strings.TrimRight(h.Config.BaseURL, "/")
	for _, v := range variants {
		urls = append(urls, fmt.Sprintf("%s/cdn/%s/%s/%s", base, accountID, imageID, v.ID))
	}
	return urls
}

// UploadImage handles POST /v1 -- multipart file upload or URL fetch.
func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		api.BadRequest(w, "invalid multipart form: "+err.Error())
		return
	}

	var (
		reader   io.Reader
		filename string
	)

	// Try file upload first.
	file, header, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		reader = file
		filename = header.Filename
	} else {
		// Try URL fetch.
		urlStr := r.FormValue("url")
		if urlStr == "" {
			api.BadRequest(w, "missing required field: file or url")
			return
		}
		resp, err := http.Get(urlStr)
		if err != nil {
			api.BadRequest(w, "failed to fetch url: "+err.Error())
			return
		}
		defer resp.Body.Close()
		reader = resp.Body

		// Derive filename from the URL path.
		parts := strings.Split(urlStr, "/")
		filename = parts[len(parts)-1]
	}

	imageID := uuid.New().String()

	// Store blob.
	_, err = h.Store.Store(accountID, imageID, reader)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to store image: "+err.Error()))
		return
	}

	// Parse optional metadata.
	var meta map[string]interface{}
	if metaStr := r.FormValue("metadata"); metaStr != "" {
		if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
			api.BadRequest(w, "invalid metadata JSON: "+err.Error())
			return
		}
	}

	requireSigned := false
	if v := r.FormValue("requireSignedURLs"); v == "true" {
		requireSigned = true
	}

	now := time.Now().UTC()
	img := &model.Image{
		ID:                imageID,
		AccountID:         accountID,
		Filename:          filename,
		Creator:           uuid.New().String(),
		Meta:              meta,
		RequireSignedURLs: requireSigned,
		Uploaded:          now,
	}

	if err := h.DB.CreateImage(img); err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to create image record: "+err.Error()))
		return
	}

	img.Variants = h.buildVariantURLs(accountID, imageID)

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(img))
}

// GetImage handles GET /v1/{image_id}.
func (h *Handler) GetImage(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	imageID := chi.URLParam(r, "image_id")

	img, err := h.DB.GetImage(accountID, imageID)
	if err != nil {
		api.NotFound(w, "image not found")
		return
	}

	img.Variants = h.buildVariantURLs(accountID, img.ID)
	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(img))
}

// ListImages handles GET /v1.
func (h *Handler) ListImages(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())

	page := 1
	perPage := 1000

	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if pp, err := strconv.Atoi(v); err == nil && pp > 0 {
			if pp > 10000 {
				pp = 10000
			}
			perPage = pp
		}
	}

	images, total, err := h.DB.ListImages(accountID, page, perPage)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to list images"))
		return
	}

	// Build variant URLs for each image.
	for _, img := range images {
		img.Variants = h.buildVariantURLs(accountID, img.ID)
	}

	// Ensure non-nil slice for JSON serialisation.
	if images == nil {
		images = []*model.Image{}
	}

	totalPages := 0
	if perPage > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	info := api.ResultInfo{
		Page:       page,
		PerPage:    perPage,
		Count:      len(images),
		TotalCount: total,
		TotalPages: totalPages,
	}

	api.WriteJSON(w, http.StatusOK, api.PaginatedResponse(map[string]interface{}{"images": images}, info))
}

// UpdateImage handles PATCH /v1/{image_id}.
func (h *Handler) UpdateImage(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	imageID := chi.URLParam(r, "image_id")

	img, err := h.DB.GetImage(accountID, imageID)
	if err != nil {
		api.NotFound(w, "image not found")
		return
	}

	var body struct {
		Metadata          *map[string]interface{} `json:"metadata"`
		RequireSignedURLs *bool                   `json:"requireSignedURLs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.BadRequest(w, "invalid JSON body: "+err.Error())
		return
	}

	if body.Metadata != nil {
		img.Meta = *body.Metadata
	}
	if body.RequireSignedURLs != nil {
		img.RequireSignedURLs = *body.RequireSignedURLs
	}

	if err := h.DB.UpdateImage(img); err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to update image"))
		return
	}

	img.Variants = h.buildVariantURLs(accountID, img.ID)
	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(img))
}

// DeleteImage handles DELETE /v1/{image_id}.
func (h *Handler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	imageID := chi.URLParam(r, "image_id")

	if err := h.DB.DeleteImage(accountID, imageID); err != nil {
		api.NotFound(w, "image not found")
		return
	}

	// Also delete the blob (best-effort).
	_ = h.Store.Delete(accountID, imageID)

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(struct{}{}))
}

// GetStats handles GET /v1/stats.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())

	count, err := h.DB.CountImages(accountID)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to count images"))
		return
	}

	result := map[string]interface{}{
		"count": map[string]interface{}{
			"current": count,
			"allowed": h.Config.ImageAllowance,
		},
	}
	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(result))
}
