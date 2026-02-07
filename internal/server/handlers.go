package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Minimal JSON types for API - server does NOT parse secret contents.
// Only validates structure enough to extract expiry/max_views for enforcement.

type dropCreateRequest struct {
	Ciphertext string `json:"ciphertext"`
	Salt       string `json:"salt"`
	IV         string `json:"iv"`
	KDF        struct {
		Algorithm string `json:"algorithm"`
		Time      uint32 `json:"time"`
		Memory    uint32 `json:"memory"`
		Threads   uint8  `json:"threads"`
	} `json:"kdf"`
	Expiry   int64 `json:"expiry"`
	MaxViews int   `json:"max_views"`
}

type dropCreateResponse struct {
	ID   string `json:"id"`
	Link string `json:"link"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func randomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// writeJSON sends a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError sends an error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// Handler returns the HTTP handler for the API.
func Handler(store *Store, baseURL string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/drop", func(w http.ResponseWriter, r *http.Request) {
		// Limit request body size to prevent DoS
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)

		// Server must never log request body
		if r.Body == nil {
			writeError(w, http.StatusBadRequest, "missing body")
			return
		}
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			if strings.Contains(err.Error(), "http: request body too large") {
				writeError(w, http.StatusRequestEntityTooLarge,
					fmt.Sprintf("request body exceeds %d MB limit", MaxRequestBodyBytes/(1024*1024)))
				return
			}
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		// Parse and validate payload
		var req dropCreateRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid payload structure")
			return
		}

		// Full server-side validation (size, expiry, max_views, required fields)
		if status, msg := validateRequest(&req); status != 0 {
			writeError(w, status, msg)
			return
		}

		expiry := time.Unix(req.Expiry, 0)
		id := randomID()
		store.Put(id, raw, req.MaxViews, expiry)

		link := baseURL + "/v1/drop/" + id
		writeJSON(w, http.StatusCreated, dropCreateResponse{ID: id, Link: link})
	})

	mux.HandleFunc("GET /v1/drop/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		blob, reason := store.GetWithReason(id)
		if blob == nil {
			switch reason {
			case ReasonExpired:
				writeError(w, http.StatusGone, "🔥 Secret expired and was automatically burned")
			case ReasonMaxViews:
				writeError(w, http.StatusGone, "🔥 Secret already retrieved and burned (max views reached)")
			case ReasonNotFound:
				writeError(w, http.StatusNotFound, "🔥 Secret not found - it may have been burned or never existed")
			default:
				writeError(w, http.StatusNotFound, "Secret not found or expired")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(blob)
	})

	mux.HandleFunc("DELETE /v1/drop/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ok := store.Delete(id)
		if !ok {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
	})

	return mux
}
