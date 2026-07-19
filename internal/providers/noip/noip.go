// Package noip 实现 No-IP 免费 DDNS 服务
// No-IP 是一个经典的动态 DNS 服务商，通过 HTTP Basic Auth 和 GET 请求更新域名解析记录
package noip

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

var log = slog.With("module", "noip")

const (
	defaultBaseURL = "https://dynupdate.no-ip.com"
	updatePath     = "/nic/update"
)

// Client No-IP DDNS API 客户端
type Client struct {
	username string
	password string
	baseURL  string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建 No-IP 客户端
func NewClient(username, password string, options ...Option) *Client {
	c := &Client{
		username: username,
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
// No-IP 无独立添加接口，调用 update 覆盖设置
func (c *Client) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	return c.update(ctx, fulldomain, value)
}

// ModifyRecord 修改域名解析记录
// No-IP 无独立修改接口，调用 update 覆盖设置
func (c *Client) ModifyRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	return c.update(ctx, fulldomain, newValue)
}

// DeleteRecord 删除域名解析记录
// No-IP 不支持删除记录，设为空操作
func (c *Client) DeleteRecord(ctx context.Context, fulldomain, recordID string) error {
	log.Debug("No-IP does not support deleting records, skipping",
		"domain", fulldomain)
	return nil
}

// GetRecords 查询域名解析记录
// No-IP 不提供记录查询 API，返回空列表
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
	log.Debug("No-IP does not support querying records, returning empty list",
		"domain", fulldomain)
	return []providers.RecordInfo{}, nil
}

// update 执行 No-IP 的 API 更新请求
func (c *Client) update(ctx context.Context, hostname, ip string) error {
	// 构建请求 URL
	reqURL := fmt.Sprintf("%s%s?hostname=%s", c.baseURL, updatePath, hostname)
	if ip != "" {
		reqURL += "&myip=" + ip
	}

	log.Debug("updating No-IP record", "hostname", hostname, "ipv6", ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("User-Agent", "ddns6/1.0 contact@notes-bin")

	resp, err := c.Do(req)
	if err != nil {
		log.Error("No-IP API request failed", "hostname", hostname, "err", err)
		return fmt.Errorf("No-IP request failed: %w", err)
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
		log.Info("No-IP record updated successfully", "hostname", hostname, "ipv6", ip)
		return nil
	case strings.HasPrefix(response, "nochg"):
		log.Debug("No-IP record unchanged", "hostname", hostname, "ipv6", ip)
		return nil
	case response == "nohost":
		return fmt.Errorf("No-IP hostname not found: %s", hostname)
	case response == "badauth":
		return fmt.Errorf("No-IP authentication failed: invalid username or password")
	case response == "badagent":
		return fmt.Errorf("No-IP bad agent: disabled User-Agent")
	case response == "!":
		return fmt.Errorf("No-IP abuse detected: too many updates")
	default:
		log.Error("No-IP API returned unexpected response",
			"hostname", hostname, "response", response)
		return fmt.Errorf("No-IP update failed: %s", response)
	}
}
