// 文件用途：验证密文随机性、解密还原、用途绑定和篡改检测。

package secretbox

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestCardEncryption 验证密文随机性、解密还原、用途绑定和篡改检测。
func TestCardEncryption(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	a, e := Seal(key, "private-card", "delivery:1:2")
	if e != nil {
		t.Fatal(e)
	}
	b, e := Seal(key, "private-card", "delivery:1:2")
	if e != nil || a == b {
		t.Fatal("nonce must be unique")
	}
	value, e := Open(key, a, "delivery:1:2")
	if e != nil || value != "private-card" {
		t.Fatal("roundtrip failed")
	}
	if _, e = Open(key, a, "delivery:2:2"); e == nil {
		t.Fatal("cipher cannot be moved across orders")
	}
	if _, e = Seal("", "card", "purpose"); e == nil {
		t.Fatal("empty key accepted")
	}
	if _, e = Open(key, a[:len(a)-4]+"AAAA", "delivery:1:2"); e == nil {
		t.Fatal("tampering accepted")
	}
}
