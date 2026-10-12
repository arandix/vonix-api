package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/vonix/api/internal/auth"
	"github.com/vonix/api/internal/models"
	"github.com/vonix/api/internal/webhook"
)

type TransferHandler struct {
	Webhooks *webhook.Store
}

func (h *TransferHandler) CreateFiatTransfer(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body is not valid JSON.")
		return
	}
	if err := validateTransferRequest(req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}

	bid := auth.BusinessID(r)
	transfer := buildTransfer(bid, "fiat_transfer", req)

	// In production: persist to DB, enqueue to payment rails, debit balance
	go h.Webhooks.Dispatch(bid, models.EventTransferCreated, transfer)
	go func() {
		time.Sleep(2 * time.Second)
		transfer.Status = models.TransferStatusCompleted
		now := time.Now().UTC()
		transfer.CompletedAt = &now
		h.Webhooks.Dispatch(bid, models.EventTransferCompleted, transfer)
	}()

	respond(w, http.StatusCreated, transfer)
}

func (h *TransferHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	bid := auth.BusinessID(r)
	now := time.Now().UTC()
	completed := now.Add(-2 * time.Minute)

	// In production: query DB with cursor-based pagination
	txns := []models.Transfer{
		{
			ID:          "txn_01j9abc",
			BusinessID:  bid,
			Type:        "fiat_transfer",
			Status:      models.TransferStatusCompleted,
			Currency:    models.CurrencyUSD,
			Amount:      "1500.00",
			Fee:         "3.50",
			Net:         "1496.50",
			Recipient:   models.Recipient{Name: "ACME Corp", IBAN: "DE89370400440532013000"},
			Reference:   "INV-2024-001",
			CreatedAt:   now.Add(-5 * time.Minute),
			CompletedAt: &completed,
		},
	}

	respond(w, http.StatusOK, models.PaginatedResponse[models.Transfer]{
		Data:       txns,
		TotalCount: len(txns),
		HasMore:    false,
	})
}

func buildTransfer(businessID, transferType string, req models.CreateTransferRequest) models.Transfer {
	return models.Transfer{
		ID:         "txn_" + randHex(6),
		BusinessID: businessID,
		Type:       transferType,
		Status:     models.TransferStatusPending,
		Currency:   req.Currency,
		Amount:     req.Amount,
		Fee:        "0.50",
		Net:        req.Amount, // simplified
		Recipient:  req.Recipient,
		Reference:  req.Reference,
		ExternalID: req.ExternalID,
		CreatedAt:  time.Now().UTC(),
	}
}

func validateTransferRequest(req models.CreateTransferRequest) error {
	if req.Amount == "" || req.Amount == "0" {
		return errors.New("amount is required and must be non-zero")
	}
	if req.Currency == "" {
		return errors.New("currency is required")
	}
	if req.Recipient.IBAN == "" && req.Recipient.Address == "" {
		return errors.New("recipient.iban or recipient.address is required")
	}
	return nil
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
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
