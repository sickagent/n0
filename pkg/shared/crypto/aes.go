package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// Encrypter provides AES-256-GCM encryption for JSON-compatible data.
type Encrypter struct {
	key []byte
}

// NewEncrypter creates an encrypter from a 32-byte key.
func NewEncrypter(key []byte) (*Encrypter, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be exactly 32 bytes for AES-256, got %d", len(key))
	}
	return &Encrypter{key: key}, nil
}

// NewEncrypterFromBase64 creates an encrypter from a base64-encoded 32-byte key.
func NewEncrypterFromBase64(keyB64 string) (*Encrypter, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode base64 key: %w", err)
	}
	return NewEncrypter(key)
}

// EncryptJSON marshals the map, encrypts with AES-256-GCM, and returns a map with a single _enc field.
func (e *Encrypter) EncryptJSON(data map[string]any) (map[string]any, error) {
	if e == nil {
		return data, nil
	}
	plaintext, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return map[string]any{"_enc": base64.StdEncoding.EncodeToString(ciphertext)}, nil
}

// DecryptJSON inspects the map for an _enc field and decrypts it; otherwise returns the map as-is.
func (e *Encrypter) DecryptJSON(data map[string]any) (map[string]any, error) {
	if e == nil {
		return data, nil
	}
	encVal, ok := data["_enc"].(string)
	if !ok {
		// Not encrypted (backward compatibility)
		return data, nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encVal)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	return result, nil
}
