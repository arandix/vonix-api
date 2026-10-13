package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vonix/api/internal/auth"
	"github.com/vonix/api/internal/models"
)

type APIKeyHandler struct {
	Store *auth.APIKeyStore
}

func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body is not valid JSON.")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "name is required")
		return
	}
	if len(req.Scopes) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "at least one scope is required")
		return
	}

	// Privilege escalation check: new key cannot have scopes the caller doesn't already have.
	callerKey, _ := r.Context().Value(auth.ContextKeyAPIKey).(*models.APIKey)
	if callerKey != nil {
		callerScopes := make(map[models.APIKeyScope]struct{}, len(callerKey.Scopes))
		for _, s := range callerKey.Scopes {
			callerScopes[s] = struct{}{}
		}
		for _, requested := range req.Scopes {
			if _, ok := callerScopes[requested]; !ok {
				writeError(w, http.StatusForbidden, "insufficient_scope",
					"Cannot grant scope '"+string(requested)+"': caller does not hold this scope.")
				return
			}
		}
	}

	// api_keys:write cannot be granted to a child key — only an admin session may do that.
	for _, s := range req.Scopes {
		if s == models.ScopeAPIKeys {
			writeError(w, http.StatusForbidden, "forbidden_scope",
				"Scope 'api_keys:write' cannot be granted via the API. Contact support.")
			return
		}
	}

	bid := auth.BusinessID(r)
	key, _, err := h.Store.Generate(bid, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate API key.")
		return
	}

	slog.Info("api_key.created",
		"business_id", bid,
		"key_id", key.ID,
		"key_name", req.Name,
		"scopes", req.Scopes,
		"caller_key_id", callerKey.ID,
	)

	// key.Key contains the plain-text value — shown ONCE
	respond(w, http.StatusCreated, key)
}

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	bid := auth.BusinessID(r)
	keys := h.Store.List(bid)
	if keys == nil {
		keys = []models.APIKey{}
	}
	respond(w, http.StatusOK, models.PaginatedResponse[models.APIKey]{
		Data:       keys,
		TotalCount: len(keys),
		HasMore:    false,
	})
}

func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	bid := auth.BusinessID(r)
	keyID := chi.URLParam(r, "keyID")
	callerKey, _ := r.Context().Value(auth.ContextKeyAPIKey).(*models.APIKey)
	if !h.Store.Revoke(bid, keyID) {
		writeError(w, http.StatusNotFound, "not_found", "API key not found or already revoked.")
		return
	}
	slog.Info("api_key.revoked",
		"business_id", bid,
		"key_id", keyID,
		"caller_key_id", callerKey.ID,
	)
	w.WriteHeader(http.StatusNoContent)
}
