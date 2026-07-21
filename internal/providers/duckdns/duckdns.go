// Package duckdns 实现 DuckDNS 免费 DDNS 服务
// DuckDNS 是一个简单的免费 DDNS 服务，通过 HTTP GET 请求更新域名解析记录
package duckdns

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
)

const (
	defaultBaseURL = "https://www.duckdns.org"
	updatePath     = "/update"
)

// Client DuckDNS API 客户端
type Client struct {
	token   string
	baseURL string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建 DuckDNS 客户端
func NewClient(token string, options ...Option) *Client {
	c := &Client{
		token:   token,
		baseURL: defaultBaseURL,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

// WithBaseURL 设置自定义 API 地址（测试用）
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.Client = httpClient
	}
}

// AddRecord 添加或更新域名解析记录
// DuckDNS 无独立添加接口，调用 update 覆盖设置
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	return c.update(ctx, record.Name, record.Value)
}

// ModifyRecord 修改域名解析记录
// DuckDNS 无独立修改接口，调用 update 覆盖设置
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	return c.update(ctx, record.Name, record.Value)
}

// DeleteRecord 删除域名解析记录
// DuckDNS 不支持删除记录，更新为空 IP 以清除
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	return c.update(ctx, record.Name, "")
}

// GetRecords 查询域名解析记录
// DuckDNS 不提供记录查询 API，返回空列表
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	slog.Debug("DuckDNS does not support querying records, returning empty list",
		"module", "duckdns",
		"domain", fulldomain)
	return []ddns.RecordInfo{}, nil
}

// update 执行 DuckDNS 的 API 更新请求
func (c *Client) update(ctx context.Context, domain, ip string) error {
	// 验证域名格式：必须为 *.duckdns.org
	if !strings.HasSuffix(domain, ".duckdns.org") {
		return fmt.Errorf("duckdns domain must end with .duckdns.org, got: %s", domain)
	}

	// 从 fulldomain 中提取 DuckDNS 域名（去掉 .duckdns.org 后缀）
	domainName := strings.TrimSuffix(domain, ".duckdns.org")
	// DuckDNS 不支持多级子域名（如 www.mydomain.duckdns.org），验证是否有额外层级
	if strings.Contains(domainName, ".") {
		slog.Warn("DuckDNS does not support multi-level subdomains, API may reject the request",
			"module", "duckdns", "domain", domain, "extracted", domainName)
	}

	query := url.Values{}
	query.Set("domains", domainName)
	query.Set("token", c.token)
	if ip != "" {
		query.Set("ipv6", ip)
	}
	// 设置 verbose 以获取明确的成功/失败响应
	query.Set("verbose", "true")

	reqURL := c.baseURL + updatePath + "?" + query.Encode()
	slog.Debug("updating DuckDNS record", "module", "duckdns", "domain", domain, "ipv6", ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		slog.Error("DuckDNS API request failed", "module", "duckdns", "domain", domain, "err", err)
		return fmt.Errorf("DuckDNS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	response := strings.TrimSpace(string(body))
	if response == "OK" {
		slog.Info("DuckDNS record updated successfully", "module", "duckdns", "domain", domain, "ipv6", ip)
		return nil
	}

	slog.Error("DuckDNS API returned unexpected response",
		"module", "duckdns",
		"domain", domain, "response", response)
	return fmt.Errorf("DuckDNS update failed: %s", response)
}
