package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

// EncryptAES256GCM encrypts plaintext with AES-256-GCM.
// hexKey must be 64 hex chars (32 bytes). Returns base64(nonce+ciphertext).
func EncryptAES256GCM(hexKey, plaintext string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return "", errors.New("invalid key: must be 64-char hex (32 bytes)")
	}
	block, err := aes.NewCipher(key)
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
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAES256GCM decrypts a base64(nonce+ciphertext) produced by EncryptAES256GCM.
func DecryptAES256GCM(hexKey, encoded string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return "", errors.New("invalid key: must be 64-char hex (32 bytes)")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
