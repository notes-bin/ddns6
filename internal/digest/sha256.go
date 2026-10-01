// Package digest 提供 HMAC/SHA256 摘要与签名辅助，供各 DNS 运营商计算请求签名。
//
// 函数均接收标准 Go 类型（[]byte），调用方按需转换编码（如 hex、base64）。
package digest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hex 计算 SHA-256 哈希并返回小写十六进制字符串。
func SHA256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// HMACSHA256 计算 HMAC-SHA256 并返回原始字节。
func HMACSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// HMACSHA256Hex 计算 HMAC-SHA256 并返回小写十六进制字符串。
func HMACSHA256Hex(key, data []byte) string {
	return hex.EncodeToString(HMACSHA256(key, data))
}
