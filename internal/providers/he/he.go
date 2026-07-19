// Package he 实现 Hurricane Electric DNS 服务
// HE DNS 提供免费的 DNS 托管服务，支持通过 DDNS API 更新 IPv6 解析记录
package he

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/providers"
)

var log = slog.With("module", "he")

const (
	defaultBaseURL = "https://dyn.dns.he.net"
	updatePath     = "/nic/update"
)

// Client HE DNS API 客户端
type Client struct {
	password string
	baseURL  string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建 HE DNS 客户端
// HE DDNS 使用固定用户名 "hosted_dns_editapi"，只需传入 DDNS Key 作为 password
func NewClient(password string, options ...Option) *Client {
	c := &Client{
		password: password,
		baseURL:  defaultBaseURL,
		Client:   &http.Client{Timeout: 10 * time.Second},
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
// HE DDNS API 使用单次更新请求覆盖记录
func (c *Client) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	return c.update(ctx, fulldomain, value)
}

// ModifyRecord 修改域名解析记录
func (c *Client) ModifyRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	return c.update(ctx, fulldomain, newValue)
}

// DeleteRecord 删除域名解析记录
// HE 不支持通过 DDNS API 删除记录
func (c *Client) DeleteRecord(ctx context.Context, fulldomain, recordID string) error {
	log.Debug("HE DNS does not support deleting records via DDNS API, skipping",
		"domain", fulldomain)
	return nil
}

// GetRecords 查询域名解析记录
// HE DDNS API 不提供记录查询接口，返回空列表
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
	log.Debug("HE DNS does not support querying records via DDNS API, returning empty list",
		"domain", fulldomain)
	return []providers.RecordInfo{}, nil
}

// update 执行 HE DNS DDNS 更新请求
func (c *Client) update(ctx context.Context, hostname, ip string) error {
	reqURL := fmt.Sprintf("%s%s?hostname=%s", c.baseURL, updatePath, hostname)
	if ip != "" {
		reqURL += "&myip=" + ip
	}

	log.Debug("updating HE DNS record", "hostname", hostname, "ipv6", ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// HE 使用固定用户名 "hosted_dns_editapi" 和 DDNS 密钥认证
	req.SetBasicAuth("hosted_dns_editapi", c.password)

	resp, err := c.Do(req)
	if err != nil {
		log.Error("HE DNS API request failed", "hostname", hostname, "err", err)
		return fmt.Errorf("HE DNS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	response := strings.TrimSpace(string(body))

	// 解析响应
	switch {
	case strings.HasPrefix(response, "good"):
		log.Info("HE DNS record updated successfully", "hostname", hostname, "ipv6", ip)
		return nil
	case strings.HasPrefix(response, "nochg"):
		log.Debug("HE DNS record unchanged", "hostname", hostname, "ipv6", ip)
		return nil
	case response == "nohost":
		return fmt.Errorf("HE DNS hostname not found: %s", hostname)
	case response == "badauth":
		return fmt.Errorf("HE DNS authentication failed: invalid DDNS key")
	case response == "badagent":
		return fmt.Errorf("HE DNS bad agent")
	case response == "!":
		return fmt.Errorf("HE DNS abuse detected")
	default:
		log.Error("HE DNS API returned unexpected response",
			"hostname", hostname, "response", response)
		return fmt.Errorf("HE DNS update failed: %s", response)
	}
}
