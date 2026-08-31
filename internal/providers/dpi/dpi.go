// Package dpi 实现 DNSPod.com 国际版 API 服务。
//
// 对应 acme.sh dns_dpi，API 基址为 https://api.dnspod.com。
// 认证方式：API ID + Key，login_token 格式为 "ID,Key"。
//
// 与 internal/providers/dnspod（dnsapi.cn 国内版）不同。
package dpi

import (
	"cmp"
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

const defaultBaseURL = "https://api.dnspod.com"

// Client DNSPod 国际版 API 客户端。
type Client struct {
	loginToken string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 DNSPod 国际版客户端。
// loginToken 格式为 "ID,Key"（与 acme.sh DPI_Id,DPI_Key 相同）。
func NewClient(loginToken string, options ...Option) *Client {
	c := &Client{
		loginToken: loginToken,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
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

type apiStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type record struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   string `json:"ttl"`
}

type recordListResponse struct {
	Status  apiStatus `json:"status"`
	Records []record  `json:"records"`
}

type recordResponse struct {
	Status apiStatus `json:"status"`
	Record struct {
		ID int `json:"id"`
	} `json:"record"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, info ddns.RecordInfo) error {
	domain, sub := domainutil.SplitDomain(info.Name, info.Zone)
	params := url.Values{
		"login_token":  {c.loginToken},
		"format":       {"json"},
		"domain":       {domain},
		"sub_domain":   {sub},
		"record_type":  {info.Type},
		"record_line":  {"default"},
		"value":        {info.Value},
		"ttl":          {strconv.Itoa(cmp.Or(info.TTL, ddns.DefaultTTL))},
	}
	slog.Debug("adding DNSPod intl record", "module", "dpi", "domain", domain, "sub", sub, "type", info.Type)
	var resp recordResponse
	if err := c.post(ctx, "/Record.Create", params, &resp); err != nil {
		return err
	}
	if resp.Status.Code != "1" {
		return fmt.Errorf("DNSPod intl API error: %s (code: %s)", resp.Status.Message, resp.Status.Code)
	}
	slog.Info("DNSPod intl record added", "module", "dpi", "domain", domain, "type", info.Type, "ipv6", info.Value)
	return nil
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, info ddns.RecordInfo) error {
	domain, sub := domainutil.SplitDomain(info.Name, info.Zone)
	params := url.Values{
		"login_token": {c.loginToken},
		"format":      {"json"},
		"domain":      {domain},
		"record_id":   {info.ID},
		"sub_domain":  {sub},
		"record_type": {info.Type},
		"record_line": {"default"},
		"value":       {info.Value},
		"ttl":         {strconv.Itoa(cmp.Or(info.TTL, ddns.DefaultTTL))},
	}
	var resp recordResponse
	if err := c.post(ctx, "/Record.Modify", params, &resp); err != nil {
		return err
	}
	if resp.Status.Code != "1" {
		return fmt.Errorf("DNSPod intl API error: %s (code: %s)", resp.Status.Message, resp.Status.Code)
	}
	return nil
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, info ddns.RecordInfo) error {
	domain, _ := domainutil.SplitDomain(info.Name, info.Zone)
	params := url.Values{
		"login_token": {c.loginToken},
		"format":      {"json"},
		"domain":      {domain},
		"record_id":   {info.ID},
	}
	var resp recordResponse
	if err := c.post(ctx, "/Record.Remove", params, &resp); err != nil {
		return err
	}
	if resp.Status.Code != "1" {
		return fmt.Errorf("DNSPod intl API error: %s (code: %s)", resp.Status.Message, resp.Status.Code)
	}
	return nil
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	domain, sub := domainutil.SplitDomain(fulldomain, "")
	params := url.Values{
		"login_token": {c.loginToken},
		"format":      {"json"},
		"domain":      {domain},
	}
	if sub != "@" {
		params.Set("sub_domain", sub)
	}
	if recordType != "" {
		params.Set("record_type", recordType)
	}
	var resp recordListResponse
	if err := c.post(ctx, "/Record.List", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status.Code != "1" {
		return nil, fmt.Errorf("DNSPod intl API error: %s (code: %s)", resp.Status.Message, resp.Status.Code)
	}
	result := make([]ddns.RecordInfo, 0, len(resp.Records))
	for _, r := range resp.Records {
		ttl, _ := strconv.Atoi(r.TTL)
		name := domain
		if r.Name != "" && r.Name != "@" {
			name = r.Name + "." + domain
		}
		result = append(result, ddns.RecordInfo{
			ID:    strconv.Itoa(r.ID),
			Name:  name,
			Zone:  domain,
			Type:  r.Type,
			Value: r.Value,
			TTL:   ttl,
		})
	}
	return result, nil
}

func (c *Client) post(ctx context.Context, path string, params url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "ddns6/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DNSPod intl API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return &httpStatusError{status: resp.StatusCode, body: string(body)}
	}
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		body = body[3:]
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// httpStatusError 表示 DNSPod 国际版 API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("DNSPod intl API error: status %d, body: %s", e.status, e.body)
}
