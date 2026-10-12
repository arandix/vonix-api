package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vonix/api/internal/models"
)

const (
	livePrefix = "vx_live_"
	testPrefix = "vx_test_"
	keyLength  = 32
)

type APIKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*storedKey // keyed by hash
}

type storedKey struct {
	key  models.APIKey
	hash string
}

func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{keys: make(map[string]*storedKey)}
}

// Generate creates a new API key; returns the record and the plain-text value (shown once).
func (s *APIKeyStore) Generate(businessID string, req models.CreateAPIKeyRequest) (*models.APIKey, string, error) {
	raw := make([]byte, keyLength)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("generate key bytes: %w", err)
	}
	plain := livePrefix + hex.EncodeToString(raw)
	hash := hashKey(plain)
	prefix := plain[:len(livePrefix)+8] + "..."

	now := time.Now().UTC()
	k := models.APIKey{
		ID:         newID(),
		BusinessID: businessID,
		Name:       req.Name,
		KeyPrefix:  prefix,
		Key:        plain, // caller reads this and we clear it on list/validate
		Scopes:     req.Scopes,
		ExpiresAt:  req.ExpiresAt,
		Active:     true,
		CreatedAt:  now,
	}

	s.mu.Lock()
	s.keys[hash] = &storedKey{key: k, hash: hash}
	s.mu.Unlock()

	return &k, plain, nil
}

// Validate authenticates a plain-text key and returns the associated record.
func (s *APIKeyStore) Validate(plain string) (*models.APIKey, bool) {
	if !strings.HasPrefix(plain, livePrefix) && !strings.HasPrefix(plain, testPrefix) {
		return nil, false
	}
	hash := hashKey(plain)

	s.mu.Lock()
	sk, ok := s.keys[hash]
	if ok && sk.key.Active {
		now := time.Now().UTC()
		sk.key.LastUsedAt = &now
	}
	s.mu.Unlock()

	if !ok || !sk.key.Active {
		return nil, false
	}
	if sk.key.ExpiresAt != nil && sk.key.ExpiresAt.Before(time.Now()) {
		return nil, false
	}
	cp := sk.key
	cp.Key = "" // never re-expose
	return &cp, true
}

// Revoke marks a key as inactive by (businessID, keyID).
func (s *APIKeyStore) Revoke(businessID, keyID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sk := range s.keys {
		if sk.key.ID == keyID && sk.key.BusinessID == businessID {
			sk.key.Active = false
			return true
		}
	}
	return false
}

// List returns all active keys for a business without the plain-text value.
func (s *APIKeyStore) List(businessID string) []models.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.APIKey
	for _, sk := range s.keys {
		if sk.key.BusinessID == businessID {
			cp := sk.key
			cp.Key = ""
			out = append(out, cp)
		}
	}
	return out
}

func hashKey(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(h[:])
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
