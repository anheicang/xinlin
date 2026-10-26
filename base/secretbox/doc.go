// Package secretbox 使用 AES-256-GCM 对私密内容进行认证加密，检测密文篡改和用途错配。
// 调用方提供 Base64 编码的 32 字节密钥，并在加解密时使用相同的 purpose。
package secretbox
