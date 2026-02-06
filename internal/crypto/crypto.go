// Package crypto provides client-side encryption for BurnEnv.
// All encryption/decryption happens locally. The server never sees plaintext.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id parameters (OWASP recommended for password hashing)
	// Time=3 provides good balance; memory=64MiB; parallelism=4
	argon2Time      = 3
	argon2Memory    = 64 * 1024 // 64 MiB
	argon2Threads   = 4
	argon2KeyLen    = 32 // AES-256
	saltSize        = 16
	gcmNonceSize    = 12
)

// KDFParams holds key derivation parameters for reproducibility.
// Stored with ciphertext so decryption can re-derive the key.
type KDFParams struct {
	Algorithm string `json:"algorithm"`
	Time      uint32 `json:"time"`
	Memory    uint32 `json:"memory"`
	Threads   uint8  `json:"threads"`
}

// EncryptedPayload is the structure stored on the server.
// Server never parses or decrypts; it stores this blob as-is.
type EncryptedPayload struct {
	Ciphertext string    `json:"ciphertext"`
	Salt       string    `json:"salt"`
	IV         string    `json:"iv"`
	KDF        KDFParams `json:"kdf"`
	Expiry     int64     `json:"expiry"`
	MaxViews   int       `json:"max_views"`
}

// Encrypt encrypts plaintext with the given password.
// Salt and IV are randomly generated per encryption.
// Returns JSON-serializable payload safe to send to server.
func Encrypt(plaintext []byte, password string) (*EncryptedPayload, error) {
	if len(plaintext) == 0 {
		return nil, errors.New("plaintext cannot be empty")
	}
	if password == "" {
		return nil, errors.New("password cannot be empty")
	}

	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Derive key from password using Argon2id (client-side only)
	key := argon2.IDKey(
		[]byte(password),
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedPayload{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Salt:       base64.StdEncoding.EncodeToString(salt),
		IV:         base64.StdEncoding.EncodeToString(nonce),
		KDF: KDFParams{
			Algorithm: "argon2id",
			Time:      argon2Time,
			Memory:    argon2Memory,
			Threads:   argon2Threads,
		},
		// Expiry and MaxViews are set by caller after encryption
	}, nil
}

// Decrypt decrypts an EncryptedPayload with the given password.
func Decrypt(payload *EncryptedPayload, password string) ([]byte, error) {
	if payload == nil {
		return nil, errors.New("payload cannot be nil")
	}
	if password == "" {
		return nil, errors.New("password cannot be empty")
	}

	salt, err := base64.StdEncoding.DecodeString(payload.Salt)
	if err != nil {
		return nil, fmt.Errorf("invalid salt: %w", err)
	}
	if len(salt) != saltSize {
		return nil, errors.New("invalid salt length")
	}

	nonce, err := base64.StdEncoding.DecodeString(payload.IV)
	if err != nil {
		return nil, fmt.Errorf("invalid IV: %w", err)
	}
	if len(nonce) != gcmNonceSize {
		return nil, errors.New("invalid IV length")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext: %w", err)
	}

	// Use KDF params from payload (allows future param evolution)
	key := argon2.IDKey(
		[]byte(password),
		salt,
		payload.KDF.Time,
		payload.KDF.Memory,
		payload.KDF.Threads,
		argon2KeyLen,
	)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("decryption failed: wrong password or corrupted data")
	}

	return plaintext, nil
}

// SerializePayload marshals the payload to JSON bytes.
func SerializePayload(p *EncryptedPayload) ([]byte, error) {
	return json.Marshal(p)
}

// DeserializePayload unmarshals JSON bytes into an EncryptedPayload.
func DeserializePayload(data []byte) (*EncryptedPayload, error) {
	var p EncryptedPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}
	if p.Ciphertext == "" || p.Salt == "" || p.IV == "" {
		return nil, errors.New("incomplete payload: missing required fields")
	}
	return &p, nil
}
