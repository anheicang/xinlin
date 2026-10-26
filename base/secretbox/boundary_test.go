package secretbox

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

type brokenRandom struct{}

func (brokenRandom) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }

func TestEncryptionRejectsInvalidKeysAndCiphertexts(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	for _, invalid := range []string{"!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := Seal(invalid, "text", "purpose"); err == nil {
			t.Fatal("invalid key accepted")
		}
		if _, err := Open(invalid, "cipher", "purpose"); err == nil {
			t.Fatal("invalid key accepted")
		}
	}
	for _, value := range []string{"!", "", base64.StdEncoding.EncodeToString([]byte("short")), base64.StdEncoding.EncodeToString(make([]byte, 12))} {
		if _, err := Open(key, value, "purpose"); err == nil {
			t.Fatal("invalid ciphertext accepted")
		}
	}
	cipher, err := Seal(key, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := Open(key, cipher, ""); err != nil || plain != "" {
		t.Fatal("empty plaintext roundtrip failed")
	}
	wrong := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("w", 32)))
	if _, err := Open(wrong, cipher, ""); err == nil {
		t.Fatal("wrong key accepted")
	}
	previous := rand.Reader
	rand.Reader = brokenRandom{}
	t.Cleanup(func() { rand.Reader = previous })
	if _, err := Seal(key, "text", "purpose"); err == nil {
		t.Fatal("entropy failure hidden")
	}
}
