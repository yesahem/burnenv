// Package client provides HTTP client for BurnEnv API.
// All crypto happens in the CLI; client only transfers encrypted blobs.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/yesahem/burnenv/internal/crypto"
)

// CreateResponse is the response from POST /v1/drop.
type CreateResponse struct {
	ID   string `json:"id"`
	Link string `json:"link"`
}

// Create sends an encrypted payload to the server and returns the link.
func Create(baseURL string, payload *crypto.EncryptedPayload) (string, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := baseURL + "/v1/drop"

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return "", fmt.Errorf("server error: %s", errResp.Error)
		}
		return "", fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var out CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Link, nil
}

// Get fetches an encrypted payload from the server (retrieve & burn).
func Get(link string) (*crypto.EncryptedPayload, error) {
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return nil, fmt.Errorf("server: %s", errResp.Error)
		}
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var p crypto.EncryptedPayload
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	if p.Ciphertext == "" || p.Salt == "" || p.IV == "" {
		return nil, fmt.Errorf("incomplete payload from server")
	}
	return &p, nil
}

// Revoke manually destroys a secret (DELETE /v1/drop/{id}).
func Revoke(link string) error {
	req, err := http.NewRequest("DELETE", link, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return fmt.Errorf("server: %s", errResp.Error)
		}
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
