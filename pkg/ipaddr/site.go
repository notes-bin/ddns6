package ipaddr

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/notes-bin/ddns6/internal/httputil"
)

// 编译期断言：HTTPIPv6Fetcher 实现 IPv6Fetcher。
var _ IPv6Fetcher = (*HTTPIPv6Fetcher)(nil)

// HTTPIPv6Fetcher 通过 HTTP GET 访问返回纯文本 IP 的端点，解析本机公网 IPv6。
//
// 客户端单次请求超时为 5 秒；总超时仍受调用方 context 约束。
type HTTPIPv6Fetcher struct {
	url    string
	client *http.Client
}

// NewHTTPIPv6Fetcher 创建指向 url 的 HTTP IPv6 获取器。
func NewHTTPIPv6Fetcher(url string) *HTTPIPv6Fetcher {
	return &HTTPIPv6Fetcher{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// String 返回目标 URL。
func (h *HTTPIPv6Fetcher) String() string {
	return h.url
}

// Fetch 请求端点并将响应正文解析为 IPv6 地址。
func (h *HTTPIPv6Fetcher) Fetch(ctx context.Context) (net.IP, error) {
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

	body, err := httputil.ReadIPBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", h.url, err)
	}

	body = bytes.TrimSpace(body)
	if bytes.Contains(body, []byte("%")) {
		body = bytes.Trim(body, "%") // 部分端点带 zone id 后缀
	}

	ip := net.ParseIP(string(body))
	if ip != nil && ip.To16() != nil && ip.To4() == nil {
		return ip, nil
	}

	respStr := string(body)
	if len(respStr) > 100 {
		respStr = respStr[:100] + "..."
	}
	slog.Warn("HTTP response is not a valid IPv6 address",
		"module", "ipaddr",
		"url", h.String(),
		"response", respStr,
	)

	return nil, fmt.Errorf("no valid ipv6 address found from %s", h.url)
}
