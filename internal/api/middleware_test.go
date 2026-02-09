package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper that returns "ok" when the request reaches the inner handler.
func okHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Errorf("okHandler: failed to write response: %v", err)
		}
	})
}

// ---------- AuthMiddleware ----------

func TestAuthMiddleware_RejectsMissingAuth(t *testing.T) {
	handler := AuthMiddleware("")(okHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp Response
	err := json.NewDecoder(w.Result().Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Len(t, resp.Errors, 1)
	assert.Equal(t, 9401, resp.Errors[0].Code)
}

func TestAuthMiddleware_AcceptsBearerToken(t *testing.T) {
	handler := AuthMiddleware("")(okHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_AcceptsXAuthKeyAndEmail(t *testing.T) {
	handler := AuthMiddleware("")(okHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Auth-Key", "my-key")
	req.Header.Set("X-Auth-Email", "user@example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_RejectsXAuthKeyWithoutEmail(t *testing.T) {
	handler := AuthMiddleware("")(okHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Auth-Key", "my-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_RejectsWrongBearerToken(t *testing.T) {
	handler := AuthMiddleware("correct-token")(okHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_AcceptsCorrectBearerToken(t *testing.T) {
	handler := AuthMiddleware("correct-token")(okHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer correct-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_RejectsWrongXAuthKey(t *testing.T) {
	handler := AuthMiddleware("correct-token")(okHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Auth-Key", "wrong-key")
	req.Header.Set("X-Auth-Email", "user@example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_AcceptsCorrectXAuthKey(t *testing.T) {
	handler := AuthMiddleware("correct-token")(okHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Auth-Key", "correct-token")
	req.Header.Set("X-Auth-Email", "user@example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------- AccountIDMiddleware ----------

func TestAccountIDMiddleware_ExtractsAccountID(t *testing.T) {
	var captured string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetAccountID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Route("/accounts/{account_id}", func(r chi.Router) {
		r.Use(AccountIDMiddleware)
		r.Get("/images", inner)
	})

	req := httptest.NewRequest(http.MethodGet, "/accounts/abc-123/images", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "abc-123", captured)
}

func TestAccountIDMiddleware_MissingAccountID(t *testing.T) {
	// When chi doesn't have the URL param, AccountIDMiddleware should return 400.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Mount without the {account_id} param to simulate a missing value.
	r := chi.NewRouter()
	r.With(AccountIDMiddleware).Get("/no-account", inner)

	req := httptest.NewRequest(http.MethodGet, "/no-account", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAccountID_EmptyContext(t *testing.T) {
	// When no account_id is in context, GetAccountID returns "".
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	assert.Equal(t, "", GetAccountID(ctx))
}
