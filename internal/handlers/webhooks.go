package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vonix/api/internal/auth"
	"github.com/vonix/api/internal/models"
	"github.com/vonix/api/internal/webhook"
)

type WebhookHandler struct {
	Store *webhook.Store
}

func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body is not valid JSON.")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "url is required")
		return
	}
	if len(req.Events) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "at least one event is required")
		return
	}

	bid := auth.BusinessID(r)
	wh, err := h.Store.Create(bid, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create webhook.")
		return
	}

	// Secret is shown only on creation
	respond(w, http.StatusCreated, wh)
}

func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	bid := auth.BusinessID(r)
	whs := h.Store.List(bid)
	if whs == nil {
		whs = []models.Webhook{}
	}
	respond(w, http.StatusOK, models.PaginatedResponse[models.Webhook]{
		Data:       whs,
		TotalCount: len(whs),
		HasMore:    false,
	})
}

func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	bid := auth.BusinessID(r)
	id := chi.URLParam(r, "webhookID")
	if !h.Store.Delete(bid, id) {
		writeError(w, http.StatusNotFound, "not_found", "Webhook not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
