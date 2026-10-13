package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vonix/api/internal/models"
)

const (
	signatureHeader   = "Vonix-Signature"
	timestampHeader   = "Vonix-Timestamp"
	apiVersion        = "2024-01"
	deliveryTimeout   = 10 * time.Second
	maxRetries        = 3
	maxTimestampAge   = 5 * time.Minute  // replay-attack window
	dedupeWindowSize  = 1024             // max in-memory dedup entries
)

type Store struct {
	mu       sync.RWMutex
	webhooks map[string]*models.Webhook
	// deduplication: tracks recently seen event payload IDs to prevent double-processing
	dedupeMu   sync.Mutex
	dedupeIDs  map[string]time.Time
}

func NewStore() *Store {
	return &Store{
		webhooks:  make(map[string]*models.Webhook),
		dedupeIDs: make(map[string]time.Time),
	}
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

	payloadID := newID()
	payload := models.WebhookPayload{
		ID:         payloadID,
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

	// Deduplicate: skip if this event ID was already dispatched recently
	if s.seenEvent(payloadID) {
		slog.Warn("webhook duplicate event skipped", "event_id", payloadID)
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

// VerifySignature verifies an incoming webhook signature.
// It validates: format, timestamp freshness (5 min), and HMAC — in that order,
// using constant-time comparison only after format/length checks pass.
func VerifySignature(secret, ts, sigHeader string, body []byte) error {
	// 1. Validate format: must start with "sha256=" followed by 64 hex chars
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return errors.New("signature must start with 'sha256='")
	}
	hexPart := sigHeader[len(prefix):]
	if len(hexPart) != 64 {
		return errors.New("signature has invalid length")
	}
	for _, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return errors.New("signature contains invalid characters")
		}
	}

	// 2. Validate timestamp: must be a unix second within ±maxTimestampAge
	epochSec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || epochSec <= 0 {
		return errors.New("timestamp is invalid")
	}
	age := time.Since(time.Unix(epochSec, 0)).Abs()
	if age > maxTimestampAge {
		return fmt.Errorf("timestamp is too old or too far in the future (age: %s)", age.Round(time.Second))
	}

	// 3. Constant-time HMAC comparison
	expected := "sha256=" + sign(secret, ts, body)
	if !hmac.Equal([]byte(expected), []byte(sigHeader)) {
		return errors.New("signature does not match")
	}
	return nil
}

// seenEvent returns true and records the ID if it has not been seen recently.
// Prunes the map when it grows beyond dedupeWindowSize.
func (s *Store) seenEvent(id string) bool {
	s.dedupeMu.Lock()
	defer s.dedupeMu.Unlock()
	cutoff := time.Now().Add(-maxTimestampAge * 2)
	if t, ok := s.dedupeIDs[id]; ok && t.After(cutoff) {
		return true
	}
	// Prune old entries when the map is large
	if len(s.dedupeIDs) >= dedupeWindowSize {
		for k, t := range s.dedupeIDs {
			if t.Before(cutoff) {
				delete(s.dedupeIDs, k)
			}
		}
	}
	s.dedupeIDs[id] = time.Now()
	return false
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
