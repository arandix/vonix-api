package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/vonix/api/internal/models"
)

const (
	keyPrefix  = "vx_live_"
	testPrefix = "vx_test_"
	keyLength  = 32
)

// APIKeyStore is an in-memory store (replace with DB in production)
type APIKeyStore struct {
	keys map[string]*storedKey
}

type storedKey struct {
	APIKey  models.APIKey
	Hash    string
}

func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{keys: make(map[string]*storedKey)}
}

// Generate creates a new API key, returning the plain-text key (shown once)
func (s *APIKeyStore) Generate(businessID string, req models.CreateAPIKeyRequest) (*models.APIKey, string, error) {
	raw := make([]byte, keyLength)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("generate key bytes: %w", err)
	}
	plain := keyPrefix + hex.EncodeToString(raw)
	hash := hashKey(plain)
	prefix := plain[:len(keyPrefix)+8] + "..."

	id := newID()
	now := time.Now().UTC()
	key := models.APIKey{
		ID:         id,
		BusinessID: businessID,
		Name:       req.Name,
		KeyPrefix:  prefix,
		Key:        plain,
		Scopes:     req.Scopes,
		ExpiresAt:  req.ExpiresAt,
		Active:     true,
		CreatedAt:  now,
	}

	s.keys[hash] = &storedKey{APIKey: key, Hash: hash}
	return &key, plain, nil
}

// Validate checks the key and returns the associated APIKey
func (s *APIKeyStore) Validate(plain string) (*models.APIKey, bool) {
	if !strings.HasPrefix(plain, keyPrefix) && !strings.HasPrefix(plain, testPrefix) {
		return nil, false
	}
	hash := hashKey(plain)
	stored, ok := s.keys[hash]
	if !ok || !stored.APIKey.Active {
		return nil, false
	}
	if stored.APIKey.ExpiresAt != nil && stored.APIKey.ExpiresAt.Before(time.Now()) {
		return nil, false
	}
	now := time.Now().UTC()
	stored.APIKey.LastUsedAt = &now
	cp := stored.APIKey
	cp.Key = "" // never expose after creation
	return &cp, true
}

// Revoke marks an API key as inactive
func (s *APIKeyStore) Revoke(businessID, keyID string) bool {
	for _, sk := range s.keys {
		if sk.APIKey.ID == keyID && sk.APIKey.BusinessID == businessID {
			sk.APIKey.Active = false
			return true
		}
	}
	return false
}

// List returns all keys for a business (without plain-text key)
func (s *APIKeyStore) List(businessID string) []models.APIKey {
	var out []models.APIKey
	for _, sk := range s.keys {
		if sk.APIKey.BusinessID == businessID {
			cp := sk.APIKey
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
