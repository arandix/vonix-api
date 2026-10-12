package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/vonix/api/internal/models"
)

type contextKey string

const (
	ContextKeyAPIKey     contextKey = "api_key"
	ContextKeyBusinessID contextKey = "business_id"
)

func Middleware(store *APIKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plain := extractBearerToken(r)
			if plain == "" {
				writeError(w, http.StatusUnauthorized, "missing_api_key", "API key required. Pass it in the Authorization header as: Bearer vx_live_...")
				return
			}

			key, ok := store.Validate(plain)
			if !ok {
				writeError(w, http.StatusUnauthorized, "invalid_api_key", "The API key is invalid, revoked, or expired.")
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyAPIKey, key)
			ctx = context.WithValue(ctx, ContextKeyBusinessID, key.BusinessID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope checks that the authenticated key has the required scope
func RequireScope(scope models.APIKeyScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, _ := r.Context().Value(ContextKeyAPIKey).(*models.APIKey)
			if key == nil {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "Authentication required.")
				return
			}
			for _, s := range key.Scopes {
				if s == scope {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusForbidden, "insufficient_scope",
				"This API key does not have the required scope: "+string(scope))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	// also accept X-API-Key header
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   http.StatusText(status),
		Code:    code,
		Message: message,
	})
}

func BusinessID(r *http.Request) string {
	v, _ := r.Context().Value(ContextKeyBusinessID).(string)
	return v
}
