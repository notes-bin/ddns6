package httputil

import "unicode/utf8"

// MaxLogErrorBytes 为写入日志的错误/响应摘要最大字节数。
const MaxLogErrorBytes = 512

// TruncateForLog 截断 s 至多 MaxLogErrorBytes，避免 API body 撑爆日志。
func TruncateForLog(s string) string {
	if len(s) <= MaxLogErrorBytes {
		return s
	}
	// 按 rune 边界截断，避免切断多字节字符
	n := MaxLogErrorBytes
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "...(truncated)"
}

// ErrForLog 返回适合日志字段的错误摘要（nil 安全）。
func ErrForLog(err error) string {
	if err == nil {
		return ""
	}
	return TruncateForLog(err.Error())
}
