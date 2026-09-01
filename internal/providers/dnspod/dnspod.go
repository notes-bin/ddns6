// Package dnspod 实现 DNSPod 旧版 API（腾讯云 DNSPod 经典接口）。
//
// 认证方式：login_token（格式 "ID,Token"）。
// 与 internal/providers/tencent（Tencent Cloud API v3）不同，
// 本包调用 dnsapi.cn 原始 form 接口。
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

// Client DNSPod 旧版 API 客户端。
type Client struct {
	loginToken string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 DNSPod 客户端。
// loginToken 格式为 "ID,Token"。
func NewClient(loginToken string, options ...Option) *Client {
	c := &Client{
		loginToken: loginToken,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
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

// dnspodStatus 为 API 响应中的 status 字段。
type dnspodStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// dnspodRecord 为 DNSPod 列表接口返回的单条记录。
type dnspodRecord struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	TTL     string `json:"ttl"`
	Enabled string `json:"enabled"`
}

// recordListResponse 为 Record.List 响应。
type recordListResponse struct {
	Status  dnspodStatus   `json:"status"`
	Records []dnspodRecord `json:"records"`
}

// recordResponse 为单条记录写操作响应。
type recordResponse struct {
	Status dnspodStatus `json:"status"`
	Record struct {
		ID int `json:"id"`
	} `json:"record"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, subDomain := domainutil.SplitDomain(record.Name, record.Zone)

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

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, subDomain := domainutil.SplitDomain(record.Name, record.Zone)

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

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, _ := domainutil.SplitDomain(record.Name, record.Zone)

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

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	domain, subDomain := domainutil.SplitDomain(fulldomain, "")

	params := url.Values{}
	params.Set("login_token", c.loginToken)
	params.Set("format", "json")
	params.Set("domain", domain)
	// subDomain 为 "@" 时不传 sub_domain，以获取该域名下全部记录
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

		// 拼完整 FQDN，确保后续 DeleteRecord 能正确拆根域名
		recordName := domain
		if r.Name != "@" && r.Name != "" {
			recordName = r.Name + "." + domain
		}
		result = append(result, ddns.RecordInfo{
			ID:    strconv.Itoa(r.ID),
			Name:  recordName,
			Type:  r.Type,
			Value: r.Value,
			TTL:   ttl,
		})
	}
	return result, nil
}

// post 以 form-urlencoded 发起 POST 并解码 JSON；会剥离响应 BOM。
func (c *Client) post(ctx context.Context, reqURL string, params url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "ddns6/1.0")

	resp, err := c.httpClient.Do(req)
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

	clean := body
	if len(clean) >= 3 && clean[0] == 0xEF && clean[1] == 0xBB && clean[2] == 0xBF {
		clean = clean[3:]
	}

	if err := json.Unmarshal(clean, result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}
