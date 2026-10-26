// 文件用途：验证无效密钥、损坏密文、空明文及随机源故障。

package secretbox

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// brokenRandom 模拟不可用随机源，验证加密失败会正确返回错误。
type brokenRandom struct{}

// Read 主动模拟读取失败，用于验证上层错误传播。
func (brokenRandom) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }

// TestEncryptionRejectsInvalidKeysAndCiphertexts 验证无效密钥、损坏密文、空明文及随机源故障。
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
