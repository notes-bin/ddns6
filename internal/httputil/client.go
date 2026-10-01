// Package httputil 提供 DNS API 调用共用的 HTTP 工具。
//
// 包含：带超时的客户端、同主机重定向限制（SameHostRedirect）、
// 响应体长度上限，以及日志/错误中的密钥脱敏（RedactSecrets）。
package httputil

import (
	"fmt"
	"net/http"
	"time"
)

// NewHTTPClient 返回带超时与同主机重定向限制的 HTTP 客户端。
//
// CheckRedirect 使用 SameHostRedirect：拒绝跨 host / 跨 scheme 重定向，
// 降低 query 凭据随重定向外泄与开放重定向 SSRF 风险。
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: SameHostRedirect,
	}
}

// SameHostRedirect 限制重定向：最多 5 次，且仅允许同 scheme+host。
//
// 跨主机或跨 scheme（如 https→http）一律拒绝。
func SameHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("stopped after 5 redirects")
	}
	if len(via) == 0 {
		return nil
	}
	orig := via[0].URL
	if req.URL.Scheme != orig.Scheme || req.URL.Host != orig.Host {
		return fmt.Errorf("refusing cross-host redirect from %s://%s to %s://%s",
			orig.Scheme, orig.Host, req.URL.Scheme, req.URL.Host)
	}
	return nil
}
