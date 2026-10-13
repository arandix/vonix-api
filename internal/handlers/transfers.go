package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/vonix/api/internal/auth"
	"github.com/vonix/api/internal/models"
	"github.com/vonix/api/internal/webhook"
)

// idempotencyStore prevents duplicate financial operations keyed by (businessID, externalID).
var (
	idempotencyMu    sync.Mutex
	idempotencyCache = map[string]models.Transfer{}
)

func idempotencyKey(bid, extID string) string { return bid + "|" + extID }

// checkOrStore returns (existing, true) if extID was already processed, else stores tx and returns (tx, false).
func checkOrStore(bid, extID string, tx models.Transfer) (models.Transfer, bool) {
	if extID == "" {
		return tx, false
	}
	k := idempotencyKey(bid, extID)
	idempotencyMu.Lock()
	defer idempotencyMu.Unlock()
	if existing, ok := idempotencyCache[k]; ok {
		return existing, true
	}
	idempotencyCache[k] = tx
	return tx, false
}

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

	// Idempotency: same external_id + same business = return original result
	if existing, dup := checkOrStore(bid, req.ExternalID, transfer); dup {
		respond(w, http.StatusOK, existing)
		return
	}

	// In production: persist to DB, enqueue to payment rails, debit balance atomically
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
		Net:        req.Amount,
		Recipient:  req.Recipient,
		Reference:  req.Reference,
		ExternalID: req.ExternalID,
		CreatedAt:  time.Now().UTC(),
	}
}

// amountRe validates: optional sign not allowed, digits with optional decimal and up to 8 places.
var amountRe = regexp.MustCompile(`^[1-9]\d{0,14}(\.\d{1,8})?$|^0\.\d{1,8}[1-9]\d*$`)

func validateAmount(amount string) error {
	if amount == "" {
		return errors.New("amount is required")
	}
	if !amountRe.MatchString(amount) {
		return fmt.Errorf("amount '%s' is invalid: must be a positive decimal with up to 8 decimal places", amount)
	}
	return nil
}

func validateTransferRequest(req models.CreateTransferRequest) error {
	if err := validateAmount(req.Amount); err != nil {
		return err
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
