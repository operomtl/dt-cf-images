package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/leca/dt-cloudflare-images/internal/api"
	"github.com/leca/dt-cloudflare-images/internal/model"
)

// CreateSigningKey handles PUT /v1/keys/{signing_key_name}.
func (h *Handler) CreateSigningKey(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	keyName := chi.URLParam(r, "signing_key_name")

	if keyName == "" {
		api.BadRequest(w, "signing key name is required")
		return
	}

	key := &model.SigningKey{
		Name:      keyName,
		Value:     uuid.New().String(),
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}

	if err := h.DB.CreateSigningKey(key); err != nil {
		api.BadRequest(w, "failed to create signing key: "+err.Error())
		return
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(key))
}

// ListSigningKeys handles GET /v1/keys.
func (h *Handler) ListSigningKeys(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())

	keys, err := h.DB.ListSigningKeys(accountID)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse(9500, "failed to list signing keys"))
		return
	}

	if keys == nil {
		keys = []*model.SigningKey{}
	}

	result := map[string]interface{}{
		"keys": keys,
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(result))
}

// DeleteSigningKey handles DELETE /v1/keys/{signing_key_name}.
func (h *Handler) DeleteSigningKey(w http.ResponseWriter, r *http.Request) {
	accountID := api.GetAccountID(r.Context())
	keyName := chi.URLParam(r, "signing_key_name")

	if err := h.DB.DeleteSigningKey(accountID, keyName); err != nil {
		api.NotFound(w, "signing key not found")
		return
	}

	// Check if this was the last key; if so, auto-create a "default" key.
	remaining, err := h.DB.ListSigningKeys(accountID)
	if err == nil && len(remaining) == 0 {
		defaultKey := &model.SigningKey{
			Name:      "default",
			Value:     uuid.New().String(),
			AccountID: accountID,
			CreatedAt: time.Now().UTC(),
		}
		_ = h.DB.CreateSigningKey(defaultKey)
	}

	api.WriteJSON(w, http.StatusOK, api.SuccessResponse(struct{}{}))
}
