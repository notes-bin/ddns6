// Package noip 实现 No-IP 免费 DDNS 更新接口。
//
// 认证方式：HTTP Basic Auth（用户名 + 密码）。
// 必填参数：--username、--password
//
// 经典 DynDNS 风格：仅 GET /nic/update；无 zone 列表、无 PathEscape 路径段、
// GetRecords 恒为空、DeleteRecord 为有意空操作（no-op）。
// HTTP 客户端默认 httputil.NewHTTPClient（含 SameHostRedirect）。
package noip

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/internal/httputil"
)

// 编译期断言：Client 实现 ddns.DNSProvider。
var _ ddns.DNSProvider = (*Client)(nil)

const (
	defaultBaseURL = "https://dynupdate.no-ip.com"
	updatePath     = "/nic/update"
)

// Client No-IP DDNS API 客户端。
type Client struct {
	username   string
	password   string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 No-IP 客户端。
func NewClient(username, password string, options ...Option) *Client {
	c := &Client{
		username:   username,
		password:   password,
		baseURL:    defaultBaseURL,
		httpClient: httputil.NewHTTPClient(10 * time.Second),
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

// WithBaseURL 设置自定义 API 地址（测试用）。
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// AddRecord 添加或更新 DNS 记录。
// No-IP 无独立添加接口，调用 update 覆盖设置。
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	return c.update(ctx, record.Name, record.Value)
}

// ModifyRecord 修改 DNS 记录。
// No-IP 无独立修改接口，调用 update 覆盖设置。
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	return c.update(ctx, record.Name, record.Value)
}

// DeleteRecord 删除 DNS 记录。
// No-IP 不支持通过 DDNS API 删除记录；此处为有意空操作（no-op），直接返回 nil。
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	slog.Debug("No-IP does not support deleting records, skipping",
		"module", "noip",
		"domain", record.Name)
	return nil
}

// GetRecords 查询 DNS 记录。
// No-IP 不提供查询 API，恒返回空列表（编排层视为“无现有记录”）。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	slog.Debug("No-IP does not support querying records, returning empty list",
		"module", "noip",
		"domain", fulldomain)
	return []ddns.RecordInfo{}, nil
}

// update 执行 No-IP DDNS 更新（hostname/myip 走 url.Values 查询串）。
func (c *Client) update(ctx context.Context, hostname, ip string) error {
	q := url.Values{}
	q.Set("hostname", hostname)
	if ip != "" {
		q.Set("myip", ip)
	}
	reqURL := c.baseURL + updatePath + "?" + q.Encode()

	slog.Debug("updating No-IP record", "module", "noip", "hostname", hostname, "ipv6", ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("User-Agent", "ddns6/1.0 contact@notes-bin")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Debug("No-IP API request failed", "module", "noip", "hostname", hostname, "err", httputil.ErrForLog(err))
		return httputil.WrapRequestError("no-ip request failed", err)
	}
	defer resp.Body.Close()

	body, err := httputil.ReadBody(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	response := strings.TrimSpace(string(body))

	// 按 No-IP 文本响应码分支：good/nochg 成功，其余映射为错误
	switch {
	case strings.HasPrefix(response, "good"):
		slog.Info("No-IP record updated successfully", "module", "noip", "hostname", hostname, "ipv6", ip)
		return nil
	case strings.HasPrefix(response, "nochg"):
		slog.Debug("No-IP record unchanged", "module", "noip", "hostname", hostname, "ipv6", ip)
		return nil
	case response == "nohost":
		return fmt.Errorf("no-ip hostname not found: %s", hostname)
	case response == "badauth":
		return fmt.Errorf("no-ip authentication failed: invalid username or password")
	case response == "badagent":
		return fmt.Errorf("no-ip bad agent: disabled user-agent")
	case response == "!":
		return fmt.Errorf("no-ip abuse detected: too many updates")
	default:
		slog.Debug("No-IP API returned unexpected response",
			"module", "noip",
			"hostname", hostname, "response", response)
		return fmt.Errorf("no-ip update failed: %s", response)
	}
}
