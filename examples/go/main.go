// Vonix Business API — Go examples
// No external dependencies needed (stdlib only)

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const baseURL = "https://api.getvonix.com"

var apiKey = os.Getenv("VONIX_API_KEY") // vx_live_...

func vonixRequest(method, path string, body any) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

// 1. Get all balances
func getBalance() (map[string]any, error) {
	return vonixRequest(http.MethodGet, "/v1/balance", nil)
}

// 2. Send a fiat payment
func sendFiatPayment(currency, amount, recipientName, iban, reference string) (map[string]any, error) {
	return vonixRequest(http.MethodPost, "/v1/transfers", map[string]any{
		"currency":    currency,
		"amount":      amount,
		"recipient":   map[string]string{"name": recipientName, "iban": iban},
		"reference":   reference,
		"external_id": fmt.Sprintf("order_%d", time.Now().Unix()),
	})
}

// 3. Get a crypto deposit address
func getDepositAddress(currency, network string) (map[string]any, error) {
	return vonixRequest(http.MethodGet,
		fmt.Sprintf("/v1/crypto/deposit-address?currency=%s&network=%s", currency, network), nil)
}

// 4. Send crypto
func sendCrypto(currency, network, amount, address string) (map[string]any, error) {
	return vonixRequest(http.MethodPost, "/v1/crypto/send", map[string]string{
		"currency": currency,
		"network":  network,
		"amount":   amount,
		"address":  address,
	})
}

// 5. Register a webhook
func registerWebhook(url string) (map[string]any, error) {
	return vonixRequest(http.MethodPost, "/v1/webhooks", map[string]any{
		"url":    url,
		"events": []string{"transfer.completed", "transfer.failed", "deposit.confirmed"},
	})
}

// 6. Verify an incoming webhook signature
func verifyWebhookSignature(secret, timestamp, signature, rawBody string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + rawBody))
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func printJSON(label string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("\n── %s\n%s\n", label, b)
}

func main() {
	bal, _ := getBalance()
	printJSON("Balance ─────────────────────────────", bal)

	transfer, _ := sendFiatPayment("USD", "250.00", "ACME Corp", "GB29NWBK60161331926819", "INV-2024-042")
	printJSON("Send Fiat ────────────────────────────", transfer)

	addr, _ := getDepositAddress("USDT", "ethereum")
	printJSON("Deposit Address (USDT/Ethereum) ──────", addr)

	send, _ := sendCrypto("USDT", "ethereum", "100.000000", "0xAbCd1234EfGh5678IjKl9012MnOp3456QrSt")
	printJSON("Send Crypto ──────────────────────────", send)
}
