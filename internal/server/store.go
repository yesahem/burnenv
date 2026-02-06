// Package server provides the BurnEnv backend API.
// The server stores only encrypted blobs. It never decrypts or sees plaintext.
package server

import (
	"sync"
	"time"
)

// NotFoundReason indicates why a secret was not found.
type NotFoundReason int

const (
	ReasonNotFound NotFoundReason = iota // Never existed or already deleted
	ReasonExpired                        // TTL expired
	ReasonMaxViews                       // Max views reached (burned)
)

// storedSecret holds an encrypted payload with view-tracking.
// Server never parses ciphertext; it stores the raw blob.
type storedSecret struct {
	Blob           []byte // Raw JSON of EncryptedPayload
	ViewsRemaining int
	Expiry         time.Time
	MaxViews       int
}

// Store is an in-memory store with TTL and max-views enforcement.
// No persistence: restart = all data lost (by design).
type Store struct {
	mu      sync.RWMutex
	secrets map[string]*storedSecret
	// Background cleanup of expired entries
	stopCleanup chan struct{}
}

// NewStore creates an in-memory store and starts TTL cleanup.
func NewStore() *Store {
	s := &Store{
		secrets:     make(map[string]*storedSecret),
		stopCleanup: make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

// Stop halts the cleanup goroutine (for graceful shutdown).
func (s *Store) Stop() {
	close(s.stopCleanup)
}

// Put stores an encrypted blob. Returns id.
func (s *Store) Put(id string, blob []byte, maxViews int, expiry time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[id] = &storedSecret{
		Blob:          blob,
		ViewsRemaining: maxViews,
		Expiry:        expiry,
		MaxViews:      maxViews,
	}
}

// Get retrieves the blob and decrements views. Deletes when views hit 0.
// Returns (blob, true) if found and not expired, (nil, false) otherwise.
func (s *Store) Get(id string) ([]byte, bool) {
	blob, _ := s.GetWithReason(id)
	return blob, blob != nil
}

// GetWithReason retrieves the blob and returns a reason if not found.
// Returns (blob, ReasonNotFound) on success (blob != nil), (nil, reason) on failure.
func (s *Store) GetWithReason(id string) ([]byte, NotFoundReason) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sec, ok := s.secrets[id]
	if !ok {
		return nil, ReasonNotFound
	}
	if time.Now().After(sec.Expiry) {
		delete(s.secrets, id)
		return nil, ReasonExpired
	}
	if sec.ViewsRemaining <= 0 {
		delete(s.secrets, id)
		return nil, ReasonMaxViews
	}
	blob := sec.Blob
	sec.ViewsRemaining--
	if sec.ViewsRemaining <= 0 {
		delete(s.secrets, id)
	}
	return blob, ReasonNotFound // Success indicated by non-nil blob
}

// Delete removes a secret (manual revoke).
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.secrets[id]
	delete(s.secrets, id)
	return ok
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCleanup:
			return
		case <-ticker.C:
			s.expireOld()
		}
	}
}

func (s *Store) expireOld() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, sec := range s.secrets {
		if now.After(sec.Expiry) {
			delete(s.secrets, id)
		}
	}
}
