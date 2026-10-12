package handlers

import (
	"encoding/json"
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

	bid := auth.BusinessID(r)
	key, _, err := h.Store.Generate(bid, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate API key.")
		return
	}

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
	if !h.Store.Revoke(bid, keyID) {
		writeError(w, http.StatusNotFound, "not_found", "API key not found or already revoked.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
