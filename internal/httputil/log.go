package httputil

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

// MaxLogErrorBytes 为写入日志的错误/响应摘要最大字节数。
const MaxLogErrorBytes = 512

// secretQueryParam 匹配 URL query 中的常见密钥参数（大小写不敏感）。
var secretQueryParam = regexp.MustCompile(`(?i)((?:token|key|apikey|api_key|secret(?:_?key)?|signature|password|accesskeyid|client_secret)=)([^&\s"']+)`)

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

// RedactSecrets 将错误/URL 字符串中的密钥类 query 参数替换为 REDACTED。
func RedactSecrets(s string) string {
	return secretQueryParam.ReplaceAllString(s, `${1}REDACTED`)
}

// SanitizeError 返回脱敏后的错误；无密钥时可原样返回。
//
// 保留 Unwrap 链，便于 errors.Is / As。
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := RedactSecrets(err.Error())
	if msg == err.Error() {
		return err
	}
	return &redactedError{msg: msg, err: err}
}

type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }

// ErrForLog 返回适合日志字段的错误摘要（nil 安全，含密钥脱敏）。
func ErrForLog(err error) string {
	if err == nil {
		return ""
	}
	return TruncateForLog(RedactSecrets(err.Error()))
}

// WrapRequestError 包装 HTTP 请求错误并脱敏 URL 中的密钥。
func WrapRequestError(op string, err error) error {
	if err == nil {
		return nil
	}
	return SanitizeError(fmt.Errorf("%s: %w", op, err))
}
