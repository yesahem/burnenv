// Package store provides a local file-based mock store for client-only testing.
// When the backend is available, the server will provide storage instead.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yesahem/burnenv/internal/crypto"
)

// storedPayload wraps the encrypted payload with view tracking metadata.
type storedPayload struct {
	Payload        *crypto.EncryptedPayload `json:"payload"`
	ViewsRemaining int                      `json:"views_remaining"`
	Expiry         int64                    `json:"expiry"`
}

// MockStore writes encrypted payloads to a local directory.
// Used for testing create/open flow without a running server.
type MockStore struct {
	dir string
}

// NewMockStore creates a mock store in the given directory.
func NewMockStore(dir string) (*MockStore, error) {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "burnenv")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &MockStore{dir: dir}, nil
}

// Save persists an encrypted payload and returns a local "link" (file path).
func (s *MockStore) Save(p *crypto.EncryptedPayload) (string, error) {
	id := randomID()
	path := filepath.Join(s.dir, id+".json")

	// Wrap payload with view tracking
	stored := storedPayload{
		Payload:        p,
		ViewsRemaining: p.MaxViews,
		Expiry:         p.Expiry,
	}
	if stored.ViewsRemaining < 1 {
		stored.ViewsRemaining = 1
	}

	data, err := json.Marshal(stored)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

// ErrSecretBurned indicates the secret was already retrieved and burned.
var ErrSecretBurned = fmt.Errorf("🔥 Secret already retrieved and burned (max views reached)")

// ErrSecretExpired indicates the secret has expired.
var ErrSecretExpired = fmt.Errorf("🔥 Secret expired and was automatically burned")

// ErrSecretNotFound indicates the secret was never created or doesn't exist.
var ErrSecretNotFound = fmt.Errorf("🔥 Secret not found - it may have been burned or never existed")

// Load retrieves an encrypted payload by path and decrements view count.
// Deletes the file when views reach 0 or secret is expired.
func (s *MockStore) Load(path string) (*crypto.EncryptedPayload, error) {
	// Sanitize: must be within our store dir
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, ErrSecretNotFound
	}
	absDir, _ := filepath.Abs(s.dir)
	if !strings.HasPrefix(absPath, absDir) {
		return nil, ErrSecretNotFound
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, ErrSecretBurned
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, ErrSecretBurned
	}

	// Try to parse as new format (with view tracking)
	var stored storedPayload
	if err := json.Unmarshal(data, &stored); err != nil {
		// Try legacy format (just the payload)
		var p crypto.EncryptedPayload
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("corrupted secret data")
		}
		// Legacy format: delete immediately
		_ = os.Remove(absPath)
		return &p, nil
	}

	// Check if payload is nil (might be legacy format parsed incorrectly)
	if stored.Payload == nil {
		var p crypto.EncryptedPayload
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("corrupted secret data")
		}
		_ = os.Remove(absPath)
		return &p, nil
	}

	// Check expiry
	if stored.Expiry > 0 && time.Now().Unix() > stored.Expiry {
		_ = os.Remove(absPath)
		return nil, ErrSecretExpired
	}

	// Check if already exhausted
	if stored.ViewsRemaining <= 0 {
		_ = os.Remove(absPath)
		return nil, ErrSecretBurned
	}

	// Decrement views
	stored.ViewsRemaining--

	// If no views remaining, delete the file
	if stored.ViewsRemaining <= 0 {
		_ = os.Remove(absPath)
	} else {
		// Update the file with decremented view count
		updatedData, err := json.Marshal(stored)
		if err == nil {
			_ = os.WriteFile(absPath, updatedData, 0600)
		}
	}

	return stored.Payload, nil
}

func randomID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(b)
}
