// 文件用途：完成 AES-GCM 加解密、随机 nonce 生成和用途绑定校验。

package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// aead 将 Base64 密钥解码并创建 AES-256-GCM 实例。
// 只接受解码后恰好 32 字节的密钥，格式或长度不符时返回错误。
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

// Seal 使用随机 nonce 加密明文，并返回包含 nonce 和认证标签的 Base64 密文。
// purpose 作为附加认证数据绑定用途，例如订单 ID；随机源失败会返回错误。
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

// Open 解码并验证密文，返回原始明文。
// 必须提供与加密时相同的 key 和 purpose；格式错误、用途错配或密文篡改均返回错误。
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
