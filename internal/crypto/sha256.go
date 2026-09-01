// Package crypto 提供密码学工具函数，供各 DNS 运营商计算签名与摘要。
//
// 函数均接收标准 Go 类型（[]byte），调用方按需转换编码。
package crypto

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
