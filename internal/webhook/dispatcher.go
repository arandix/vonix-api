package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/vonix/api/internal/models"
)

const (
	signatureHeader = "Vonix-Signature"
	timestampHeader = "Vonix-Timestamp"
	apiVersion      = "2024-01"
	deliveryTimeout = 10 * time.Second
	maxRetries      = 3
)

type Store struct {
	mu       sync.RWMutex
	webhooks map[string]*models.Webhook
}

func NewStore() *Store {
	return &Store{webhooks: make(map[string]*models.Webhook)}
}

func (s *Store) Create(businessID string, req models.CreateWebhookRequest) (*models.Webhook, error) {
	secret, err := generateSecret()
	if err != nil {
		return nil, err
	}
	wh := &models.Webhook{
		ID:         newID(),
		BusinessID: businessID,
		URL:        req.URL,
		Events:     req.Events,
		Secret:     secret,
		Active:     true,
		CreatedAt:  time.Now().UTC(),
	}
	s.mu.Lock()
	s.webhooks[wh.ID] = wh
	s.mu.Unlock()
	return wh, nil
}

func (s *Store) List(businessID string) []models.Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Webhook
	for _, wh := range s.webhooks {
		if wh.BusinessID == businessID && wh.Active {
			cp := *wh
			cp.Secret = "" // never re-expose secret
			out = append(out, cp)
		}
	}
	return out
}

func (s *Store) Delete(businessID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	wh, ok := s.webhooks[id]
	if !ok || wh.BusinessID != businessID {
		return false
	}
	wh.Active = false
	return true
}

// Dispatch sends a webhook event to all matching endpoints asynchronously
func (s *Store) Dispatch(businessID string, event models.WebhookEvent, data any) {
	s.mu.RLock()
	var targets []*models.Webhook
	for _, wh := range s.webhooks {
		if wh.BusinessID == businessID && wh.Active && hasEvent(wh.Events, event) {
			cp := *wh
			targets = append(targets, &cp)
		}
	}
	s.mu.RUnlock()

	if len(targets) == 0 {
		return
	}

	payload := models.WebhookPayload{
		ID:         newID(),
		Event:      event,
		APIVersion: apiVersion,
		CreatedAt:  time.Now().UTC(),
		Data:       data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhook marshal", "err", err)
		return
	}

	for _, wh := range targets {
		go deliver(wh, body)
	}
}

func deliver(wh *models.Webhook, body []byte) {
	client := &http.Client{Timeout: deliveryTimeout}
	for attempt := range maxRetries {
		ts := fmt.Sprintf("%d", time.Now().Unix())
		sig := sign(wh.Secret, ts, body)

		req, err := http.NewRequest(http.MethodPost, wh.URL, bytes.NewReader(body))
		if err != nil {
			slog.Error("webhook request build", "err", err, "url", wh.URL)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(timestampHeader, ts)
		req.Header.Set(signatureHeader, "sha256="+sig)
		req.Header.Set("User-Agent", "Vonix-Webhooks/1.0")

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 300 {
				return
			}
		}

		backoff := time.Duration(1<<attempt) * time.Second
		slog.Warn("webhook delivery failed", "url", wh.URL, "attempt", attempt+1, "retry_in", backoff)
		time.Sleep(backoff)
	}
	slog.Error("webhook permanently failed", "url", wh.URL)
}

// sign creates an HMAC-SHA256 signature: sha256=hex(hmac(secret, "ts.body"))
func sign(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies an incoming webhook signature (for re-emitting)
func VerifySignature(secret, ts, sigHeader string, body []byte) bool {
	expected := "sha256=" + sign(secret, ts, body)
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

func generateSecret() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(b), nil
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func hasEvent(events []models.WebhookEvent, target models.WebhookEvent) bool {
	for _, e := range events {
		if e == target || e == "*" {
			return true
		}
	}
	return strings.Contains(string(target), ".") &&
		contains(events, models.WebhookEvent(strings.Split(string(target), ".")[0]+".*"))
}

func contains(events []models.WebhookEvent, target models.WebhookEvent) bool {
	for _, e := range events {
		if e == target {
			return true
		}
	}
	return false
}
