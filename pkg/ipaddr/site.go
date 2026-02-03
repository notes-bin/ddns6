package ipaddr

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// HttpIPv6Fetcher 从 HTTP 端点获取 IPv6 地址
type HttpIPv6Fetcher string

// NewHttpIPv6Fetcher 创建新的 HttpIPv6Fetcher
func NewHttpIPv6Fetcher(url string) *HttpIPv6Fetcher {
	return (*HttpIPv6Fetcher)(&url)
}

// String 返回 HttpIPv6Fetcher 的字符串表示
func (h *HttpIPv6Fetcher) String() string {
	return string(*h)
}

// Fetch 实现 Fetcher 接口
func (h *HttpIPv6Fetcher) Fetch(ctx context.Context) (net.IP, error) {
	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(*h), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", *h, err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", *h, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", *h, err)
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

	return nil, fmt.Errorf("no valid IPv6 address found from %s", *h)
}
