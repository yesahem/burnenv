package server

import (
	"fmt"
	"net/http"
	"time"
)

// Server-side size and input validation limits.
const (
	// Size limits
	MaxRequestBodyBytes = 2 * 1024 * 1024 // 2 MB total request body
	MaxCiphertextLen    = 2 * 1024 * 1024 // 1.5 MB base64 ciphertext string (~1.5 MB decoded)
	MaxSaltLen          = 64              // base64-encoded salt
	MaxIVLen            = 64              // base64-encoded IV

	// Expiry limits (server-side; client TUI uses 2-10 min)
	MinExpirySeconds = 60           // 1 minute minimum
	MaxExpirySeconds = 24 * 60 * 60 // 24 hours maximum

	// Max views limits (server-side; client TUI uses 1-5)
	MinMaxViews = 1
	MaxMaxViews = 100
)

// validateRequest performs full server-side validation of the create request.
// Returns an HTTP status code and error message if validation fails.
func validateRequest(req *dropCreateRequest) (int, string) {
	// --- Required fields ---
	if req.Ciphertext == "" {
		return http.StatusBadRequest, "missing required field: ciphertext"
	}
	if req.Salt == "" {
		return http.StatusBadRequest, "missing required field: salt"
	}
	if req.IV == "" {
		return http.StatusBadRequest, "missing required field: iv"
	}

	// --- Size limits ---
	if len(req.Ciphertext) > MaxCiphertextLen {
		return http.StatusRequestEntityTooLarge,
			fmt.Sprintf("ciphertext exceeds maximum size (limit: %.1f MB)", float64(MaxCiphertextLen)/(1024*1024))
	}
	if len(req.Salt) > MaxSaltLen {
		return http.StatusBadRequest,
			fmt.Sprintf("salt exceeds maximum length (%d bytes)", MaxSaltLen)
	}
	if len(req.IV) > MaxIVLen {
		return http.StatusBadRequest,
			fmt.Sprintf("iv exceeds maximum length (%d bytes)", MaxIVLen)
	}

	// --- Expiry validation ---
	now := time.Now().Unix()
	if req.Expiry <= now {
		return http.StatusBadRequest, "expiry must be in the future"
	}
	duration := req.Expiry - now
	if duration < MinExpirySeconds {
		return http.StatusBadRequest,
			fmt.Sprintf("expiry must be at least %d seconds from now", MinExpirySeconds)
	}
	if duration > MaxExpirySeconds {
		return http.StatusBadRequest,
			fmt.Sprintf("expiry cannot exceed %d hours from now", MaxExpirySeconds/3600)
	}

	// --- Max views validation ---
	if req.MaxViews < MinMaxViews {
		return http.StatusBadRequest,
			fmt.Sprintf("max_views must be at least %d", MinMaxViews)
	}
	if req.MaxViews > MaxMaxViews {
		return http.StatusBadRequest,
			fmt.Sprintf("max_views cannot exceed %d", MaxMaxViews)
	}

	return 0, ""
}
