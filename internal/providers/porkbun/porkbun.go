// Package porkbun 实现 Porkbun DNS API 服务
// Porkbun 是一个流行的域名注册商，提供 RESTful JSON API 管理 DNS 记录
package porkbun

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

	"github.com/notes-bin/ddns6/internal/providers"
)

var log = slog.With("module", "porkbun")

const (
	defaultBaseURL = "https://api.porkbun.com/api/json/v3/dns"
)

// Client Porkbun DNS API 客户端
type Client struct {
	apiKey       string
	secretAPIKey string
	baseURL      string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建 Porkbun 客户端
func NewClient(apiKey, secretAPIKey string, options ...Option) *Client {
	c := &Client{
		apiKey:       apiKey,
		secretAPIKey: secretAPIKey,
		baseURL:      defaultBaseURL,
		Client:       &http.Client{Timeout: 10 * time.Second},
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

// DNSRecord Porkbun DNS 记录
type DNSRecord struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	Content string `json:"content"`
	TTL     int    `json:"ttl,omitempty"`
}

// apiResponse Porkbun API 通用响应
type apiResponse struct {
	Status  string          `json:"status"`
	Records []DNSRecord     `json:"records,omitempty"`
}

// AddRecord 添加域名解析记录
func (c *Client) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	domain, subDomain := splitDomain(fulldomain)

	record := DNSRecord{
		Name:    subDomain,
		Type:    recordType,
		Content: value,
		TTL:     ttl,
	}

	url := fmt.Sprintf("%s/create/%s", c.baseURL, domain)
	log.Debug("adding Porkbun DNS record", "domain", domain, "name", subDomain, "type", recordType)

	var resp apiResponse
	err := c.post(ctx, url, &record, &resp)
	if err != nil {
		return err
	}
	if resp.Status != "SUCCESS" {
		return fmt.Errorf("Porkbun API error: status %s", resp.Status)
	}

	log.Info("Porkbun DNS record added successfully", "domain", domain, "name", subDomain, "ipv6", value)
	return nil
}

// ModifyRecord 修改域名解析记录
// Porkbun 使用 editByNameType 接口按名称和类型修改记录
func (c *Client) ModifyRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	domain, subDomain := splitDomain(fulldomain)

	record := DNSRecord{
		Content: newValue,
		TTL:     ttl,
	}

	url := fmt.Sprintf("%s/editByNameType/%s/%s/%s", c.baseURL, domain, recordType, subDomain)
	log.Debug("modifying Porkbun DNS record", "domain", domain, "name", subDomain, "type", recordType)

	var resp apiResponse
	err := c.post(ctx, url, &record, &resp)
	if err != nil {
		return err
	}
	if resp.Status != "SUCCESS" {
		return fmt.Errorf("Porkbun API error: status %s", resp.Status)
	}

	log.Info("Porkbun DNS record modified successfully", "domain", domain, "name", subDomain, "ipv6", newValue)
	return nil
}

// DeleteRecord 删除域名解析记录
// Porkbun 使用 deleteByNameType 接口
func (c *Client) DeleteRecord(ctx context.Context, fulldomain, recordID string) error {
	domain, subDomain := splitDomain(fulldomain)

	url := fmt.Sprintf("%s/deleteByNameType/%s/%s/%s", c.baseURL, domain, "AAAA", subDomain)
	log.Debug("deleting Porkbun DNS record", "domain", domain, "name", subDomain)

	var resp apiResponse
	err := c.post(ctx, url, nil, &resp)
	if err != nil {
		return err
	}
	if resp.Status != "SUCCESS" {
		return fmt.Errorf("Porkbun API error: status %s", resp.Status)
	}

	log.Info("Porkbun DNS record deleted successfully", "domain", domain, "name", subDomain)
	return nil
}

// GetRecords 查询域名解析记录
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
	domain, subDomain := splitDomain(fulldomain)

	// Porkbun 的 retrieveByNameType 会按传入的记录类型和名称过滤
	url := fmt.Sprintf("%s/retrieveByNameType/%s/%s/%s", c.baseURL, domain, recordType, subDomain)
	log.Debug("querying Porkbun DNS records", "domain", domain, "name", subDomain, "type", recordType)

	var resp apiResponse
	err := c.post(ctx, url, nil, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != "SUCCESS" {
		return nil, fmt.Errorf("Porkbun API error: status %s", resp.Status)
	}

	result := make([]providers.RecordInfo, 0, len(resp.Records))
	for _, r := range resp.Records {
		result = append(result, providers.RecordInfo{
			Name:  r.Name,
			Type:  r.Type,
			Value: r.Content,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// apiRequest Porkbun API 请求体（自动注入 API Key + Secret Key）
type apiRequest struct {
	APIKey       string `json:"apikey"`
	SecretAPIKey string `json:"secretapikey"`
	*DNSRecord
}

// post 执行 POST JSON 请求，自动注入认证信息
func (c *Client) post(ctx context.Context, url string, record *DNSRecord, result any) error {
	apiReq := apiRequest{
		APIKey:       c.apiKey,
		SecretAPIKey: c.secretAPIKey,
		DNSRecord:    record,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("Porkbun API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Porkbun API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// splitDomain 将完整域名分割为根域名和子域名
func splitDomain(fulldomain string) (string, string) {
	return providers.SplitDomain(fulldomain)
}
