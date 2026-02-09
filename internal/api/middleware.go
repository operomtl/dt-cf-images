package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type contextKey string

const accountIDKey contextKey = "account_id"

// AuthMiddleware returns middleware that validates authentication.
// If token is empty, any request carrying a Bearer token or X-Auth-Key+X-Auth-Email
// header pair is accepted. If token is non-empty, the Bearer token must match exactly.
func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Bearer token first.
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				const prefix = "Bearer "
				if strings.HasPrefix(authHeader, prefix) {
					bearerValue := authHeader[len(prefix):]
					if token == "" || bearerValue == token {
						next.ServeHTTP(w, r)
						return
					}
					// Token provided but doesn't match.
					Unauthorized(w)
					return
				}
			}

			// Try X-Auth-Key + X-Auth-Email.
			authKey := r.Header.Get("X-Auth-Key")
			authEmail := r.Header.Get("X-Auth-Email")
			if authKey != "" && authEmail != "" {
				if token == "" || authKey == token {
					next.ServeHTTP(w, r)
					return
				}
				// Token provided but doesn't match.
				Unauthorized(w)
				return
			}

			// No valid auth headers found.
			Unauthorized(w)
		})
	}
}

// AccountIDMiddleware extracts the account_id from the chi URL parameter
// and stores it in the request context.
func AccountIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID := chi.URLParam(r, "account_id")
		if accountID == "" {
			BadRequest(w, "account_id is required")
			return
		}
		ctx := context.WithValue(r.Context(), accountIDKey, accountID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAccountID retrieves the account_id stored in the context by AccountIDMiddleware.
func GetAccountID(ctx context.Context) string {
	v, _ := ctx.Value(accountIDKey).(string)
	return v
}
