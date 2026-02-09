package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leca/dt-cloudflare-images/internal/api"
	"github.com/leca/dt-cloudflare-images/internal/model"
)

// validFitModes lists the allowed values for the variant "fit" option.
var validFitModes = map[string]bool{
	"scale-down": true,
	"contain":    true,
	"cover":      true,
	"crop":       true,
	"pad":        true,
}

// maxVariantsPerAccount is the Cloudflare Images limit on variants.
const maxVariantsPerAccount = 100

// createVariantRequest is the JSON body for creating a variant.
type createVariantRequest struct {
	ID                     string                `json:"id"`
	Options                model.VariantOptions   `json:"options"`
	NeverRequireSignedURLs bool                   `json:"neverRequireSignedURLs"`
}

// updateVariantRequest is the JSON body for updating a variant.
type updateVariantRequest struct {
	Options                *model.VariantOptions `json:"options,omitempty"`
	NeverRequireSignedURLs *bool                 `json:"neverRequireSignedURLs,omitempty"`
}

// CreateVariant handles POST /v1/variants.
func (h *Handler) CreateVariant(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())

	var req createVariantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "invalid JSON body")
		return
	}

	if req.ID == "" {
		api.BadRequest(w, "variant id is required")
		return
	}

	if !validFitModes[req.Options.Fit] {
		api.BadRequest(w, "invalid fit mode: must be one of scale-down, contain, cover, crop, pad")
		return
	}

	// Check variant count limit.
	count, err := h.DB.CountVariants(accountID)
	if err != nil {
		api.BadRequest(w, "failed to count variants")
		return
	}
	if count >= maxVariantsPerAccount {
		api.BadRequest(w, "maximum number of variants reached")
		return
	}

	variant := &model.Variant{
		ID:                     req.ID,
		AccountID:              accountID,
		Options:                req.Options,
		NeverRequireSignedURLs: req.NeverRequireSignedURLs,
	}

	if err := h.DB.CreateVariant(variant); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			api.Conflict(w, "variant already exists")
			return
		}
		api.BadRequest(w, "failed to create variant")
		return
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(variant))
}

// ListVariants handles GET /v1/variants.
func (h *Handler) ListVariants(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())

	variants, err := h.DB.ListVariants(accountID)
	if err != nil {
		api.BadRequest(w, "failed to list variants")
		return
	}

	// Build a map keyed by variant ID, as the Cloudflare API returns.
	variantMap := make(map[string]*model.Variant)
	for _, v := range variants {
		variantMap[v.ID] = v
	}

	result := map[string]interface{}{
		"variants": variantMap,
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(result))
}

// GetVariant handles GET /v1/variants/{variant_id}.
func (h *Handler) GetVariant(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	variantID := chi.URLParam(r, "variant_id")

	variant, err := h.DB.GetVariant(accountID, variantID)
	if err != nil {
		api.NotFound(w, "variant not found")
		return
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(variant))
}

// UpdateVariant handles PATCH /v1/variants/{variant_id}.
func (h *Handler) UpdateVariant(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	variantID := chi.URLParam(r, "variant_id")

	existing, err := h.DB.GetVariant(accountID, variantID)
	if err != nil {
		api.NotFound(w, "variant not found")
		return
	}

	var req updateVariantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.BadRequest(w, "invalid JSON body")
		return
	}

	if req.Options != nil {
		if req.Options.Fit != "" && !validFitModes[req.Options.Fit] {
			api.BadRequest(w, "invalid fit mode: must be one of scale-down, contain, cover, crop, pad")
			return
		}
		if req.Options.Fit != "" {
			existing.Options.Fit = req.Options.Fit
		}
		if req.Options.Width != 0 {
			existing.Options.Width = req.Options.Width
		}
		if req.Options.Height != 0 {
			existing.Options.Height = req.Options.Height
		}
		if req.Options.Metadata != "" {
			existing.Options.Metadata = req.Options.Metadata
		}
	}

	if req.NeverRequireSignedURLs != nil {
		existing.NeverRequireSignedURLs = *req.NeverRequireSignedURLs
	}

	if err := h.DB.UpdateVariant(existing); err != nil {
		api.BadRequest(w, "failed to update variant")
		return
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(existing))
}

// DeleteVariant handles DELETE /v1/variants/{variant_id}.
func (h *Handler) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	variantID := chi.URLParam(r, "variant_id")

	if err := h.DB.DeleteVariant(accountID, variantID); err != nil {
		api.NotFound(w, "variant not found")
		return
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(struct{}{}))
}
