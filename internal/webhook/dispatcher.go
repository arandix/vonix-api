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
	"net"
	"net/http"
	"net/url"
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
	// SSRF prevention: block internal IPs and non-HTTPS
	if err := validateWebhookURL(req.URL); err != nil {
		return nil, err
	}
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
			cp.Secret = ""
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

// Dispatch sends the event to all matching endpoints asynchronously.
func (s *Store) Dispatch(businessID string, event models.WebhookEvent, data any) {
	s.mu.RLock()
	var targets []models.Webhook
	for _, wh := range s.webhooks {
		if wh.BusinessID == businessID && wh.Active && matchesEvent(wh.Events, event) {
			targets = append(targets, *wh)
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
		slog.Error("webhook marshal failed", "err", err)
		return
	}

	for _, wh := range targets {
		wh := wh
		go deliver(&wh, body)
	}
}

func deliver(wh *models.Webhook, body []byte) {
	client := &http.Client{Timeout: deliveryTimeout}
	for attempt := range maxRetries {
		ts := fmt.Sprintf("%d", time.Now().Unix())
		sig := sign(wh.Secret, ts, body)

		req, err := http.NewRequest(http.MethodPost, wh.URL, bytes.NewReader(body))
		if err != nil {
			slog.Error("webhook request build failed", "err", err)
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
			slog.Warn("webhook endpoint returned error", "url", wh.URL, "status", resp.StatusCode)
		} else {
			slog.Warn("webhook delivery failed", "url", wh.URL, "err", err)
		}

		backoff := time.Duration(1<<attempt) * time.Second
		time.Sleep(backoff)
	}
	slog.Error("webhook permanently failed after retries", "url", wh.URL)
}

func sign(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies an incoming webhook using the stored secret.
func VerifySignature(secret, ts, sigHeader string, body []byte) bool {
	expected := "sha256=" + sign(secret, ts, body)
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

// matchesEvent checks if the event matches any registered pattern.
// Supports exact match ("transfer.completed") and wildcard ("transfer.*").
func matchesEvent(registered []models.WebhookEvent, event models.WebhookEvent) bool {
	for _, r := range registered {
		if r == event {
			return true
		}
		// wildcard: "transfer.*" matches "transfer.completed"
		if strings.HasSuffix(string(r), ".*") {
			prefix := strings.TrimSuffix(string(r), ".*")
			if strings.HasPrefix(string(event), prefix+".") {
				return true
			}
		}
	}
	return false
}

// validateWebhookURL blocks SSRF by rejecting non-HTTPS and private/loopback IPs.
func validateWebhookURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("webhook url is invalid: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook url must use HTTPS")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook url has no host")
	}
	// Resolve and block private/loopback addresses
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("webhook host could not be resolved: %w", err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("webhook url must not point to a private or loopback address")
		}
	}
	return nil
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
