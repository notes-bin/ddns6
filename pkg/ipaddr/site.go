package ipaddr

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// HttpIPv6Fetcher 通过 HTTP GET 访问返回纯文本 IP 的端点，解析本机公网 IPv6。
//
// 客户端单次请求超时为 5 秒；总超时仍受调用方 context 约束。
type HttpIPv6Fetcher struct {
	url    string
	client *http.Client
}

// NewHttpIPv6Fetcher 创建指向 url 的 HTTP IPv6 获取器。
func NewHttpIPv6Fetcher(url string) *HttpIPv6Fetcher {
	return &HttpIPv6Fetcher{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// String 返回目标 URL。
func (h *HttpIPv6Fetcher) String() string {
	return h.url
}

// Fetch 请求端点并将响应正文解析为 IPv6 地址。
func (h *HttpIPv6Fetcher) Fetch(ctx context.Context) (net.IP, error) {
	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", h.url, err)
	}

	slog.Debug("fetching IPv6 via HTTP", "module", "ipaddr", "url", h.url)

	client := h.client

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", h.url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", h.url, err)
	}

	// 清理响应内容
	body = bytes.TrimSpace(body)
	if bytes.Contains(body, []byte("%")) {
		body = bytes.Trim(body, "%")
	}

	// 解析 IPv6 地址
	ip := net.ParseIP(string(body))
	if ip != nil && ip.To16() != nil && ip.To4() == nil {
		return ip, nil
	}

	// 截断响应内容避免日志过长
	respStr := string(body)
	if len(respStr) > 100 {
		respStr = respStr[:100] + "..."
	}
	slog.Warn("HTTP response is not a valid IPv6 address",
		"module", "ipaddr",
		"url", h.String(),
		"response", respStr,
	)

	return nil, fmt.Errorf("no valid IPv6 address found from %s", h.url)
}
