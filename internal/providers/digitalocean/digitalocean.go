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

	"github.com/notes-bin/ddns6/internal/providers"
)

var log = slog.With("module", "digitalocean")

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
func (c *Client) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	domain := extractDomain(fulldomain)

	record := DomainRecord{
		Type: recordType,
		Name: extractSubDomain(fulldomain),
		Data: value,
		TTL:  ttl,
	}

	url := fmt.Sprintf("%s/domains/%s/records", c.baseURL, domain)
	body, err := json.Marshal(map[string]DomainRecord{"data": record})
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	log.Debug("adding DigitalOcean DNS record", "domain", domain, "type", recordType)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("DigitalOcean API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DigitalOcean API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	log.Info("DigitalOcean DNS record added successfully", "domain", domain, "type", recordType, "ipv6", value)
	return nil
}

// ModifyRecord 修改域名解析记录
func (c *Client) ModifyRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	domain := extractDomain(fulldomain)

	record := DomainRecord{
		Type: recordType,
		Data: newValue,
		TTL:  ttl,
	}

	url := fmt.Sprintf("%s/domains/%s/records/%s", c.baseURL, domain, recordID)
	body, err := json.Marshal(map[string]DomainRecord{"data": record})
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	log.Debug("modifying DigitalOcean DNS record", "domain", domain, "record_id", recordID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("DigitalOcean API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DigitalOcean API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	log.Info("DigitalOcean DNS record modified successfully", "domain", domain, "record_id", recordID, "ipv6", newValue)
	return nil
}

// DeleteRecord 删除域名解析记录
func (c *Client) DeleteRecord(ctx context.Context, fulldomain, recordID string) error {
	domain := extractDomain(fulldomain)

	url := fmt.Sprintf("%s/domains/%s/records/%s", c.baseURL, domain, recordID)
	log.Debug("deleting DigitalOcean DNS record", "domain", domain, "record_id", recordID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("DigitalOcean API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DigitalOcean API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	log.Info("DigitalOcean DNS record deleted successfully", "domain", domain, "record_id", recordID)
	return nil
}

// GetRecords 查询域名解析记录
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
	domain := extractDomain(fulldomain)

	url := fmt.Sprintf("%s/domains/%s/records?per_page=200", c.baseURL, domain)
	log.Debug("querying DigitalOcean DNS records", "domain", domain, "type", recordType)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DigitalOcean API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DigitalOcean API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var apiResult struct {
		DomainRecords []DomainRecord `json:"domain_records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := make([]providers.RecordInfo, 0, len(apiResult.DomainRecords))
	for _, r := range apiResult.DomainRecords {
		if recordType != "" && r.Type != recordType {
			continue
		}
		result = append(result, providers.RecordInfo{
			ID:    fmt.Sprintf("%d", r.ID),
			Name:  r.Name,
			Type:  r.Type,
			Value: r.Data,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// extractDomain 从完整域名中提取根域名（假设最后两部分为根域名）
func extractDomain(fulldomain string) string {
	parts := strings.Split(fulldomain, ".")
	if len(parts) < 2 {
		return fulldomain
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// extractSubDomain 从完整域名中提取子域名前缀
func extractSubDomain(fulldomain string) string {
	parts := strings.Split(fulldomain, ".")
	if len(parts) <= 2 {
		return "@"
	}
	return strings.Join(parts[:len(parts)-2], ".")
}

// setAuth 设置 Bearer Token 认证头
func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
}
