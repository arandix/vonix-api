package models

import "time"

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyGBP Currency = "EUR"
	CurrencyEUR Currency = "GBP"
	CurrencyBTC Currency = "BTC"
	CurrencyETH Currency = "ETH"
	CurrencyUSDT Currency = "USDT"
	CurrencyBNB Currency = "BNB"
)

type AccountType string

const (
	AccountTypeFiat   AccountType = "fiat"
	AccountTypeCrypto AccountType = "crypto"
)

type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusFailed    TransferStatus = "failed"
)

type WebhookEvent string

const (
	EventTransferCreated   WebhookEvent = "transfer.created"
	EventTransferCompleted WebhookEvent = "transfer.completed"
	EventTransferFailed    WebhookEvent = "transfer.failed"
	EventDepositReceived   WebhookEvent = "deposit.received"
	EventDepositConfirmed  WebhookEvent = "deposit.confirmed"
)

// APIKeyScope defines the permissions of an API key
type APIKeyScope string

const (
	ScopeRead          APIKeyScope = "accounts:read"
	ScopeTransfer      APIKeyScope = "transfers:write"
	ScopeCrypto        APIKeyScope = "crypto:write"
	ScopeWebhooks      APIKeyScope = "webhooks:write"
)

type Balance struct {
	Currency  Currency `json:"currency"`
	Available string   `json:"available"`
	Pending   string   `json:"pending"`
	Total     string   `json:"total"`
}

type Account struct {
	ID          string      `json:"id"`
	BusinessID  string      `json:"business_id"`
	Type        AccountType `json:"type"`
	Currency    Currency    `json:"currency"`
	IBAN        string      `json:"iban,omitempty"`
	Network     string      `json:"network,omitempty"`
	Balance     Balance     `json:"balance"`
	CreatedAt   time.Time   `json:"created_at"`
}

type Transfer struct {
	ID            string         `json:"id"`
	BusinessID    string         `json:"business_id"`
	Type          string         `json:"type"`
	Status        TransferStatus `json:"status"`
	Currency      Currency       `json:"currency"`
	Amount        string         `json:"amount"`
	Fee           string         `json:"fee"`
	Net           string         `json:"net"`
	Recipient     Recipient      `json:"recipient"`
	Reference     string         `json:"reference,omitempty"`
	ExternalID    string         `json:"external_id,omitempty"`
	Network       string         `json:"network,omitempty"`
	TxHash        string         `json:"tx_hash,omitempty"`
	Confirmations int            `json:"confirmations,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

type Recipient struct {
	Name    string `json:"name,omitempty"`
	IBAN    string `json:"iban,omitempty"`
	Address string `json:"address,omitempty"`
	Network string `json:"network,omitempty"`
}

type DepositAddress struct {
	Currency  Currency  `json:"currency"`
	Network   string    `json:"network"`
	Address   string    `json:"address"`
	Memo      string    `json:"memo,omitempty"`
	QRCode    string    `json:"qr_code"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Webhook struct {
	ID         string         `json:"id"`
	BusinessID string         `json:"business_id"`
	URL        string         `json:"url"`
	Events     []WebhookEvent `json:"events"`
	Secret     string         `json:"secret,omitempty"`
	Active     bool           `json:"active"`
	CreatedAt  time.Time      `json:"created_at"`
}

type WebhookPayload struct {
	ID          string       `json:"id"`
	Event       WebhookEvent `json:"event"`
	APIVersion  string       `json:"api_version"`
	CreatedAt   time.Time    `json:"created_at"`
	Data        any          `json:"data"`
}

type APIKey struct {
	ID         string        `json:"id"`
	BusinessID string        `json:"business_id"`
	Name       string        `json:"name"`
	KeyPrefix  string        `json:"key_prefix"`
	Key        string        `json:"key,omitempty"` // only shown on creation
	Scopes     []APIKeyScope `json:"scopes"`
	LastUsedAt *time.Time    `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time    `json:"expires_at,omitempty"`
	Active     bool          `json:"active"`
	CreatedAt  time.Time     `json:"created_at"`
}

// Request/Response types

type CreateTransferRequest struct {
	Currency   Currency  `json:"currency"`
	Amount     string    `json:"amount"`
	Recipient  Recipient `json:"recipient"`
	Reference  string    `json:"reference,omitempty"`
	ExternalID string    `json:"external_id,omitempty"`
}

type SendCryptoRequest struct {
	Currency   Currency `json:"currency"`
	Network    string   `json:"network"`
	Amount     string   `json:"amount"`
	Address    string   `json:"address"`
	Memo       string   `json:"memo,omitempty"`
	ExternalID string   `json:"external_id,omitempty"`
}

type CreateWebhookRequest struct {
	URL    string         `json:"url"`
	Events []WebhookEvent `json:"events"`
}

type CreateAPIKeyRequest struct {
	Name      string        `json:"name"`
	Scopes    []APIKeyScope `json:"scopes"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
}

type PaginatedResponse[T any] struct {
	Data       []T    `json:"data"`
	TotalCount int    `json:"total_count"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
