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

// metadataFilter represents a single metadata filter parsed from query params.
type metadataFilter struct {
	Key   string
	Op    string
	Value string
}

// parseMetadataFilters extracts metadata filters from query parameters.
// Filters have the form metadata[key][op]=value where op is one of:
// eq, ne, lt, gt, lte, gte.
func parseMetadataFilters(r *http.Request) ([]metadataFilter, error) {
	var filters []metadataFilter

	for param, values := range r.URL.Query() {
		if !strings.HasPrefix(param, "metadata[") {
			continue
		}

		// Parse metadata[key][op] format.
		rest := strings.TrimPrefix(param, "metadata[")
		closeBracket := strings.Index(rest, "]")
		if closeBracket < 0 {
			continue
		}
		key := rest[:closeBracket]
		rest = rest[closeBracket+1:]

		if !strings.HasPrefix(rest, "[") || !strings.HasSuffix(rest, "]") {
			continue
		}
		op := rest[1 : len(rest)-1]

		switch op {
		case "eq", "ne", "lt", "gt", "lte", "gte":
			// valid
		default:
			return nil, fmt.Errorf("unsupported metadata filter operator: %s", op)
		}

		if len(values) > 0 {
			filters = append(filters, metadataFilter{
				Key:   key,
				Op:    op,
				Value: values[0],
			})
		}
	}

	return filters, nil
}

// ListImagesV2 handles GET /v2 -- cursor-based pagination with optional metadata filtering.
func (h *Handler) ListImagesV2(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())

	// Parse per_page (default 20, max 100).
	perPage := 20
	if v := r.URL.Query().Get("per_page"); v != "" {
		if pp, err := strconv.Atoi(v); err == nil && pp > 0 {
			if pp > 100 {
				pp = 100
			}
			perPage = pp
		}
	}

	// Parse sort_order (default "asc").
	sortOrder := "asc"
	if v := r.URL.Query().Get("sort_order"); v != "" {
		if strings.EqualFold(v, "desc") {
			sortOrder = "desc"
		}
	}

	// Parse continuation_token (cursor).
	cursor := r.URL.Query().Get("continuation_token")

	// Parse metadata filters.
	filters, err := parseMetadataFilters(r)
	if err != nil {
		api.BadRequest(w, err.Error())
		return
	}
	if len(filters) > 5 {
		api.BadRequest(w, "too many metadata filters (max 5)")
		return
	}

	var images []*model.Image
	var nextCursor string

	if len(filters) > 0 {
		// Use first filter only for now.
		f := filters[0]
		images, nextCursor, err = h.DB.ListImagesWithFilter(accountID, cursor, perPage, sortOrder, f.Key, f.Op, f.Value)
	} else {
		images, nextCursor, err = h.DB.ListImagesV2(accountID, cursor, perPage, sortOrder)
	}
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

	result := map[string]interface{}{
		"images":             images,
		"continuation_token": nextCursor,
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(result))
}

// CreateDirectUpload handles POST /v2/direct_upload.
func (h *Handler) CreateDirectUpload(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())

	// Parse request body -- support both JSON and form-encoded.
	var expiry time.Time
	var metadata map[string]interface{}

	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		var body struct {
			Expiry   string                 `json:"expiry"`
			Metadata map[string]interface{} `json:"metadata"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			api.BadRequest(w, "invalid JSON body: "+err.Error())
			return
		}
		if body.Expiry != "" {
			parsed, err := time.Parse(time.RFC3339, body.Expiry)
			if err != nil {
				api.BadRequest(w, "invalid expiry format, use RFC3339")
				return
			}
			expiry = parsed
		}
		metadata = body.Metadata
	} else {
		// Try form-encoded / multipart.
		_ = r.ParseMultipartForm(1 << 20)
		if v := r.FormValue("expiry"); v != "" {
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				api.BadRequest(w, "invalid expiry format, use RFC3339")
				return
			}
			expiry = parsed
		}
		if v := r.FormValue("metadata"); v != "" {
			if err := json.Unmarshal([]byte(v), &metadata); err != nil {
				api.BadRequest(w, "invalid metadata JSON: "+err.Error())
				return
			}
		}
	}

	// Default expiry: 30 minutes from now.
	if expiry.IsZero() {
		expiry = time.Now().UTC().Add(30 * time.Minute)
	}

	uploadID := uuid.New().String()
	baseURL := strings.TrimRight(h.Config.BaseURL, "/")
	uploadURL := fmt.Sprintf("%s/upload/%s", baseURL, uploadID)

	du := &model.DirectUpload{
		ID:        uploadID,
		AccountID: accountID,
		UploadURL: uploadURL,
		Expiry:    expiry,
		Metadata:  metadata,
		Completed: false,
	}

	if err := h.DB.CreateDirectUpload(du); err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to create direct upload: "+err.Error()))
		return
	}

	result := map[string]interface{}{
		"id":        uploadID,
		"uploadURL": uploadURL,
	}
	resp := map[string]interface{}{
		"result":      result,
		"success":     true,
		"errors":      []api.APIError{},
		"messages":    []api.APIMessage{},
		"result_info": nil,
	}
	api.WriteJSON(w, http.StatusOK, resp)
}

// HandleDirectUpload handles POST /upload/{upload_id} -- the public upload endpoint.
func (h *Handler) HandleDirectUpload(w http.ResponseWriter, r *http.Request) {
	uploadID := chi.URLParam(r, "upload_id")
	if uploadID == "" {
		api.BadRequest(w, "upload_id is required")
		return
	}

	du, err := h.DB.GetDirectUpload(uploadID)
	if err != nil {
		api.NotFound(w, "direct upload not found")
		return
	}

	// Check expiry.
	if time.Now().UTC().After(du.Expiry) {
		api.BadRequest(w, "upload URL has expired")
		return
	}

	// Check already completed.
	if du.Completed {
		api.Conflict(w, "upload already completed")
		return
	}

	// Parse multipart form for file.
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		api.BadRequest(w, "invalid multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		api.BadRequest(w, "missing required field: file")
		return
	}
	defer file.Close()

	// Use the upload ID as the image ID.
	imageID := uploadID
	accountID := du.AccountID

	// Store the file.
	_, err = h.Store.Store(accountID, imageID, file)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to store image: "+err.Error()))
		return
	}

	// Create image record in DB.
	now := time.Now().UTC()
	img := &model.Image{
		ID:        imageID,
		AccountID: accountID,
		Filename:  header.Filename,
		Creator:   uuid.New().String(),
		Meta:      du.Metadata,
		Uploaded:  now,
		Draft:     true,
	}

	if err := h.DB.CreateImage(img); err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to create image record: "+err.Error()))
		return
	}

	// If the direct upload had metadata, set it in the metadata table too.
	if len(du.Metadata) > 0 {
		if err := h.DB.SetImageMetadata(accountID, imageID, du.Metadata); err != nil {
			// Non-fatal: log but continue.
			_ = err
		}
	}

	// Mark direct upload as completed.
	if err := h.DB.CompleteDirectUpload(uploadID); err != nil {
		// Non-fatal: the image is already created.
		_ = err
	}

	img.Variants = h.buildVariantURLs(accountID, imageID)
	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(img))
}
