// Package secretbox encrypts private fulfillment content with authenticated encryption.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

func aead(key string) (cipher.AEAD, error) {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(b) != 32 {
		return nil, errors.New("virtual goods encryption key must be 32 bytes encoded as base64")
	}
	block, err := aes.NewCipher(b)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func Seal(key, plaintext, purpose string) (string, error) {
	a, err := aead(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(a.Seal(nonce, nonce, []byte(plaintext), []byte(purpose))), nil
}
func Open(key, ciphertext, purpose string) (string, error) {
	a, err := aead(key)
	if err != nil {
		return "", err
	}
	b, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil || len(b) < a.NonceSize() {
		return "", errors.New("invalid encrypted content")
	}
	plain, err := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte(purpose))
	return string(plain), err
}
