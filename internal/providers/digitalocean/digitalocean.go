// Package digitalocean 实现 DigitalOcean DNS API 服务
// DigitalOcean 提供 RESTful JSON API 管理 DNS 记录，使用 Bearer Token 认证
package digitalocean

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

const (
	defaultBaseURL = "https://api.digitalocean.com/v2"
)

// Client DigitalOcean DNS API 客户端
type Client struct {
	token   string
	baseURL string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建 DigitalOcean DNS 客户端
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

// DomainRecord DigitalOcean DNS 记录
type DomainRecord struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	Priority int    `json:"priority,omitempty"`
	Port     int    `json:"port,omitempty"`
	TTL      int    `json:"ttl"`
}

// AddRecord 添加域名解析记录
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, subDomain := domainutil.SplitDomain(record.Name, record.Zone)

	dnsRecord := DomainRecord{
		Type: record.Type,
		Name: subDomain,
		Data: record.Value,
		TTL:  record.TTL,
	}

	body, err := json.Marshal(dnsRecord)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	url := fmt.Sprintf("%s/domains/%s/records", c.baseURL, domain)
	slog.Debug("adding DigitalOcean DNS record", "module", "digitalocean", "domain", domain, "type", record.Type)

	respBody, err := c.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}

	// DigitalOcean 返回 201 Created 或 200 OK
	_ = respBody // 响应体不需要解析

	slog.Info("DigitalOcean DNS record added successfully", "module", "digitalocean", "domain", domain, "type", record.Type, "ipv6", record.Value)
	return nil
}

// ModifyRecord 修改域名解析记录
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, _ := domainutil.SplitDomain(record.Name, record.Zone)

	dnsRecord := DomainRecord{
		Type: record.Type,
		Data: record.Value,
		TTL:  record.TTL,
	}

	body, err := json.Marshal(dnsRecord)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	url := fmt.Sprintf("%s/domains/%s/records/%s", c.baseURL, domain, record.ID)
	slog.Debug("modifying DigitalOcean DNS record", "module", "digitalocean", "domain", domain, "record_id", record.ID)

	_, err = c.doRequest(ctx, http.MethodPut, url, body)
	if err != nil {
		return err
	}

	slog.Info("DigitalOcean DNS record modified successfully", "module", "digitalocean", "domain", domain, "record_id", record.ID, "ipv6", record.Value)
	return nil
}

// DeleteRecord 删除域名解析记录
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, _ := domainutil.SplitDomain(record.Name, record.Zone)

	url := fmt.Sprintf("%s/domains/%s/records/%s", c.baseURL, domain, record.ID)
	slog.Debug("deleting DigitalOcean DNS record", "module", "digitalocean", "domain", domain, "record_id", record.ID)

	_, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	slog.Info("DigitalOcean DNS record deleted successfully", "module", "digitalocean", "domain", domain, "record_id", record.ID)
	return nil
}

// GetRecords 查询域名解析记录
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	domain, _ := domainutil.SplitDomain(fulldomain, "")

	url := fmt.Sprintf("%s/domains/%s/records?per_page=200", c.baseURL, domain)
	slog.Debug("querying DigitalOcean DNS records", "module", "digitalocean", "domain", domain, "type", recordType)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var apiResult struct {
		DomainRecords []DomainRecord `json:"domain_records"`
	}
	if err := json.Unmarshal(respBody, &apiResult); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := make([]ddns.RecordInfo, 0, len(apiResult.DomainRecords))
	for _, r := range apiResult.DomainRecords {
		if recordType != "" && r.Type != recordType {
			continue
		}

		// 构建完整记录名（含根域名），确保后续 DeleteRecord 能正确提取根域名
		recordName := domain
		if r.Name != "@" && r.Name != "" {
			recordName = r.Name + "." + domain
		}
		result = append(result, ddns.RecordInfo{
			ID:    fmt.Sprintf("%d", r.ID),
			Name:  recordName,
			Type:  r.Type,
			Value: r.Data,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// setAuth 设置 Bearer Token 认证头
func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
}

// doRequest 执行 HTTP 请求，检查 2xx 状态码并返回响应体
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DigitalOcean API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("DigitalOcean API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
