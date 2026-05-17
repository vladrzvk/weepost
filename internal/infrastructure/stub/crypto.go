package stub

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// CryptoService implémente domain.ICryptoService via AES-256-GCM.
type CryptoService struct {
	key []byte
}

func NewCryptoService(key string) (*CryptoService, error) {
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		// not base64 — treat as raw bytes (legacy / dev)
		k = []byte(key)
	}
	if len(k) != 32 {
		return nil, fmt.Errorf("CRYPTO_KEY must be exactly 32 bytes, got %d", len(k))
	}
	return &CryptoService{key: k}, nil
}

func (s *CryptoService) EncryptToken(ctx context.Context, plaintext string) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func (s *CryptoService) DecryptToken(ctx context.Context, ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext encoding: %w", err)
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	return string(plaintext), nil
}
