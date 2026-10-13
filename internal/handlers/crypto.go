package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vonix/api/internal/auth"
	"github.com/vonix/api/internal/models"
	"github.com/vonix/api/internal/webhook"
)

type CryptoHandler struct {
	Webhooks *webhook.Store
}

var supportedNetworks = map[models.Currency][]string{
	models.CurrencyBTC:  {"bitcoin"},
	models.CurrencyETH:  {"ethereum"},
	models.CurrencyUSDT: {"ethereum", "tron", "bnb_smart_chain"},
	models.CurrencyBNB:  {"bnb_smart_chain"},
}

// GetDepositAddress returns a unique deposit address for the business
func (h *CryptoHandler) GetDepositAddress(w http.ResponseWriter, r *http.Request) {
	currency := models.Currency(strings.ToUpper(r.URL.Query().Get("currency")))
	network := strings.ToLower(r.URL.Query().Get("network"))

	networks, ok := supportedNetworks[currency]
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported_currency",
			fmt.Sprintf("Unsupported currency: %s. Supported: BTC, ETH, USDT, BNB", currency))
		return
	}
	if network == "" {
		network = networks[0]
	}

	// In production: generate or return deterministic address from custody provider (e.g. BitGo)
	addr := generateDemoAddress(currency, network, auth.BusinessID(r))

	respond(w, http.StatusOK, models.DepositAddress{
		Currency: currency,
		Network:  network,
		Address:  addr,
		QRCode:   fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?data=%s&size=200x200", addr),
	})
}

// SendCrypto initiates a crypto withdrawal
func (h *CryptoHandler) SendCrypto(w http.ResponseWriter, r *http.Request) {
	var req models.SendCryptoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body is not valid JSON.")
		return
	}
	if err := validateCryptoSend(req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}

	bid := auth.BusinessID(r)
	transfer := models.Transfer{
		ID:         "txn_" + randHex(6),
		BusinessID: bid,
		Type:       "crypto_send",
		Status:     models.TransferStatusPending,
		Currency:   req.Currency,
		Amount:     req.Amount,
		Fee:        cryptoFee(req.Currency),
		Recipient:  models.Recipient{Address: req.Address, Network: req.Network},
		ExternalID: req.ExternalID,
		Network:    req.Network,
		CreatedAt:  time.Now().UTC(),
	}

	// Idempotency: same external_id + same business = return original result
	if existing, dup := checkOrStore(bid, req.ExternalID, transfer); dup {
		respond(w, http.StatusOK, existing)
		return
	}

	go h.Webhooks.Dispatch(bid, models.EventTransferCreated, transfer)
	go func() {
		time.Sleep(30 * time.Second) // production: wait for blockchain confirmation
		transfer.Status = models.TransferStatusCompleted
		transfer.TxHash = "0x" + randHex(32)
		transfer.Confirmations = 12
		now := time.Now().UTC()
		transfer.CompletedAt = &now
		h.Webhooks.Dispatch(bid, models.EventTransferCompleted, transfer)
	}()

	respond(w, http.StatusCreated, transfer)
}

// ListSupportedAssets returns all supported crypto assets and networks
func ListSupportedAssets(w http.ResponseWriter, r *http.Request) {
	type Asset struct {
		Currency models.Currency `json:"currency"`
		Name     string          `json:"name"`
		Networks []string        `json:"networks"`
		Decimals int             `json:"decimals"`
	}
	assets := []Asset{
		{Currency: models.CurrencyBTC, Name: "Bitcoin", Networks: supportedNetworks[models.CurrencyBTC], Decimals: 8},
		{Currency: models.CurrencyETH, Name: "Ethereum", Networks: supportedNetworks[models.CurrencyETH], Decimals: 18},
		{Currency: models.CurrencyUSDT, Name: "Tether USD", Networks: supportedNetworks[models.CurrencyUSDT], Decimals: 6},
		{Currency: models.CurrencyBNB, Name: "BNB", Networks: supportedNetworks[models.CurrencyBNB], Decimals: 18},
	}
	respond(w, http.StatusOK, map[string]any{"assets": assets})
}

func validateCryptoSend(req models.SendCryptoRequest) error {
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.Network == "" {
		return fmt.Errorf("network is required")
	}
	if req.Address == "" {
		return fmt.Errorf("address is required")
	}
	if err := validateAmount(req.Amount); err != nil {
		return err
	}
	networks, ok := supportedNetworks[req.Currency]
	if !ok {
		return fmt.Errorf("unsupported currency: %s", req.Currency)
	}
	for _, n := range networks {
		if n == req.Network {
			return nil
		}
	}
	return fmt.Errorf("network %s is not supported for %s", req.Network, req.Currency)
}

func cryptoFee(c models.Currency) string {
	switch c {
	case models.CurrencyBTC:
		return "0.00005000"
	case models.CurrencyETH:
		return "0.001500000000000000"
	default:
		return "2.000000"
	}
}

func generateDemoAddress(currency models.Currency, network, businessID string) string {
	// Production: call BitGo/Fireblocks to get deterministic address
	seed := randHex(16)
	switch currency {
	case models.CurrencyBTC:
		return "bc1q" + seed[:28]
	case models.CurrencyETH, models.CurrencyUSDT:
		if network == "tron" {
			return "T" + seed[:33]
		}
		return "0x" + seed + randHex(4)
	default:
		return "0x" + seed + randHex(4)
	}
}
