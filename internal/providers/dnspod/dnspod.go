// Package dnspod 实现 DNSPod 旧版 API（腾讯云 DNSPod 经典接口）
// 与 internal/providers/tencent（Tencent Cloud API v3）不同，此包使用 DNSPod 的原始 API
package dnspod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

const (
	defaultBaseURL = "https://dnsapi.cn"
)

// Client DNSPod 旧版 API 客户端
type Client struct {
	loginToken string
	baseURL    string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建 DNSPod 客户端
// loginToken 格式为 "ID,Token"
func NewClient(loginToken string, options ...Option) *Client {
	c := &Client{
		loginToken: loginToken,
		baseURL:    defaultBaseURL,
		Client:     &http.Client{Timeout: 10 * time.Second},
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

// dnspodStatus DNSPod API 响应状态
type dnspodStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// dnspodRecord DNSPod 记录
type dnspodRecord struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	TTL     string `json:"ttl"`
	Enabled string `json:"enabled"`
}

// recordListResponse DNSPod 记录列表响应
type recordListResponse struct {
	Status  dnspodStatus   `json:"status"`
	Records []dnspodRecord `json:"records"`
}

// recordResponse DNSPod 单条记录操作响应
type recordResponse struct {
	Status dnspodStatus `json:"status"`
	Record struct {
		ID int `json:"id"`
	} `json:"record"`
}

// AddRecord 添加域名解析记录
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, subDomain := splitDomain(record.Name)

	params := url.Values{}
	params.Set("login_token", c.loginToken)
	params.Set("format", "json")
	params.Set("domain", domain)
	params.Set("sub_domain", subDomain)
	params.Set("record_type", record.Type)
	params.Set("record_line", "默认")
	params.Set("value", record.Value)
	params.Set("ttl", strconv.Itoa(record.TTL))

	url := c.baseURL + "/Record.Create"
	slog.Debug("adding DNSPod record", "module", "dnspod", "domain", domain, "subdomain", subDomain, "type", record.Type)

	var resp recordResponse
	if err := c.post(ctx, url, params, &resp); err != nil {
		return err
	}
	if resp.Status.Code != "1" {
		return fmt.Errorf("DNSPod API error: %s (code: %s)", resp.Status.Message, resp.Status.Code)
	}

	slog.Info("DNSPod record added successfully", "module", "dnspod", "domain", domain, "subdomain", subDomain, "ipv6", record.Value)
	return nil
}

// ModifyRecord 修改域名解析记录
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, subDomain := splitDomain(record.Name)

	params := url.Values{}
	params.Set("login_token", c.loginToken)
	params.Set("format", "json")
	params.Set("domain", domain)
	params.Set("record_id", record.ID)
	params.Set("sub_domain", subDomain)
	params.Set("record_type", record.Type)
	params.Set("record_line", "默认")
	params.Set("value", record.Value)
	params.Set("ttl", strconv.Itoa(record.TTL))

	url := c.baseURL + "/Record.Modify"
	slog.Debug("modifying DNSPod record", "module", "dnspod", "domain", domain, "record_id", record.ID)

	var resp recordResponse
	if err := c.post(ctx, url, params, &resp); err != nil {
		return err
	}
	if resp.Status.Code != "1" {
		return fmt.Errorf("DNSPod API error: %s (code: %s)", resp.Status.Message, resp.Status.Code)
	}

	slog.Info("DNSPod record modified successfully", "module", "dnspod", "domain", domain, "record_id", record.ID, "ipv6", record.Value)
	return nil
}

// DeleteRecord 删除域名解析记录
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, _ := splitDomain(record.Name)

	params := url.Values{}
	params.Set("login_token", c.loginToken)
	params.Set("format", "json")
	params.Set("domain", domain)
	params.Set("record_id", record.ID)

	url := c.baseURL + "/Record.Remove"
	slog.Debug("deleting DNSPod record", "module", "dnspod", "domain", domain, "record_id", record.ID)

	var resp recordResponse
	if err := c.post(ctx, url, params, &resp); err != nil {
		return err
	}
	if resp.Status.Code != "1" {
		return fmt.Errorf("DNSPod API error: %s (code: %s)", resp.Status.Message, resp.Status.Code)
	}

	slog.Info("DNSPod record deleted successfully", "module", "dnspod", "domain", domain, "record_id", record.ID)
	return nil
}

// GetRecords 查询域名解析记录
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	domain, subDomain := splitDomain(fulldomain)

	params := url.Values{}
	params.Set("login_token", c.loginToken)
	params.Set("format", "json")
	params.Set("domain", domain)
	// subDomain 为 "@" 时不传 sub_domain，获取该域名下所有记录
	if subDomain != "@" {
		params.Set("sub_domain", subDomain)
	}
	if recordType != "" {
		params.Set("record_type", recordType)
	}

	url := c.baseURL + "/Record.List"
	slog.Debug("querying DNSPod records", "module", "dnspod", "domain", domain, "subdomain", subDomain, "type", recordType)

	var resp recordListResponse
	if err := c.post(ctx, url, params, &resp); err != nil {
		return nil, err
	}
	if resp.Status.Code != "1" {
		return nil, fmt.Errorf("DNSPod API error: %s (code: %s)", resp.Status.Message, resp.Status.Code)
	}

	result := make([]ddns.RecordInfo, 0, len(resp.Records))
	for _, r := range resp.Records {
		ttl, _ := strconv.Atoi(r.TTL)
		result = append(result, ddns.RecordInfo{
			ID:    strconv.Itoa(r.ID),
			Name:  r.Name,
			Type:  r.Type,
			Value: r.Value,
			TTL:   ttl,
		})
	}
	return result, nil
}

// post 执行 POST form-data 请求并解码 JSON 响应
func (c *Client) post(ctx context.Context, reqURL string, params url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "ddns6/1.0")

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("DNSPod API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DNSPod API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// 去除可能的 BOM 前缀
	clean := body
	if len(clean) >= 3 && clean[0] == 0xEF && clean[1] == 0xBB && clean[2] == 0xBF {
		clean = clean[3:]
	}

	if err := json.Unmarshal(clean, result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// splitDomain 分割完整域名为根域名和子域名
func splitDomain(fulldomain string) (string, string) {
	return domainutil.SplitDomain(fulldomain)
}
