// Package dynv6 实现 Dynv6 免费 DDNS 服务
// Dynv6 是一个专为 IPv6 设计的免费动态 DNS 服务商，提供 RESTful JSON API
package dynv6

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

var log = slog.With("module", "dynv6")

const (
	defaultBaseURL = "https://dynv6.com"
)

// Client Dynv6 API 客户端
type Client struct {
	token   string
	baseURL string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建 Dynv6 客户端
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

// Zone Dynv6 区域信息
type Zone struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	IPv4  string `json:"ipv4address"`
	IPv6  string `json:"ipv6address"`
}

// Record Dynv6 DNS 记录
type Record struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Data string `json:"data"`
	TTL  int    `json:"ttl,omitempty"`
}

// AddRecord 添加域名解析记录
func (c *Client) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	zoneID, subDomain, err := c.resolveZone(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to resolve zone: %w", err)
	}

	// 如果是主域名（无子域名），直接更新 zone 的 IPv6 地址
	if subDomain == "" || subDomain == "@" {
		return c.updateZoneIP(ctx, zoneID, value)
	}

	record := Record{
		Type: recordType,
		Name: subDomain,
		Data: value,
	}
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/zones/%s/records", c.baseURL, zoneID)
	log.Debug("adding Dynv6 record", "zone_id", zoneID, "name", subDomain, "type", recordType)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("Dynv6 API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Dynv6 API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Info("Dynv6 record added successfully", "zone_id", zoneID, "name", subDomain, "ipv6", value)
	return nil
}

// ModifyRecord 修改域名解析记录
func (c *Client) ModifyRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	zoneID, _, err := c.resolveZone(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to resolve zone: %w", err)
	}

	// PATCH 方式更新记录
	record := Record{
		Type: recordType,
		Data: newValue,
		TTL:  ttl,
	}
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/zones/%s/records/%s", c.baseURL, zoneID, recordID)
	log.Debug("modifying Dynv6 record", "zone_id", zoneID, "record_id", recordID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("Dynv6 API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Dynv6 API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Info("Dynv6 record modified successfully", "zone_id", zoneID, "record_id", recordID, "ipv6", newValue)
	return nil
}

// DeleteRecord 删除域名解析记录
func (c *Client) DeleteRecord(ctx context.Context, fulldomain, recordID string) error {
	zoneID, _, err := c.resolveZone(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to resolve zone: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/zones/%s/records/%s", c.baseURL, zoneID, recordID)
	log.Debug("deleting Dynv6 record", "zone_id", zoneID, "record_id", recordID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("Dynv6 API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Dynv6 API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Info("Dynv6 record deleted successfully", "zone_id", zoneID, "record_id", recordID)
	return nil
}

// GetRecords 查询域名解析记录
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
	zoneID, subDomain, err := c.resolveZone(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve zone: %w", err)
	}

	// 主域名：检查 zone 的 IPv6 地址
	if subDomain == "" || subDomain == "@" {
		zone, err := c.getZone(ctx, zoneID)
		if err != nil {
			return nil, err
		}
		if zone.IPv6 != "" {
			return []providers.RecordInfo{
				{ID: zoneID, Name: zone.Name, Type: "AAAA", Value: zone.IPv6},
			}, nil
		}
		return []providers.RecordInfo{}, nil
	}

	// 子域名：查询 records
	url := fmt.Sprintf("%s/api/v2/zones/%s/records", c.baseURL, zoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Dynv6 API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Dynv6 API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var records []Record
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := make([]providers.RecordInfo, 0, len(records))
	for _, r := range records {
		if recordType != "" && r.Type != recordType {
			continue
		}
		result = append(result, providers.RecordInfo{
			ID:    r.ID,
			Name:  r.Name,
			Type:  r.Type,
			Value: r.Data,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// resolveZone 解析域名对应的 zone ID 和子域名
func (c *Client) resolveZone(ctx context.Context, domain string) (string, string, error) {
	// 获取所有 zones
	url := c.baseURL + "/api/v2/zones"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	c.setAuth(req)

	resp, err := c.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to list zones: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to list zones, status: %d", resp.StatusCode)
	}

	var zones []Zone
	if err := json.NewDecoder(resp.Body).Decode(&zones); err != nil {
		return "", "", fmt.Errorf("failed to decode zones: %w", err)
	}

	// 从右到左匹配 zone 名称
	parts := strings.Split(domain, ".")
	for i := 0; i < len(parts); i++ {
		zoneName := strings.Join(parts[i:], ".")
		for _, z := range zones {
			if z.Name == zoneName {
				subDomain := ""
				if i > 0 {
					subDomain = strings.Join(parts[:i], ".")
				}
				log.Debug("resolved Dynv6 zone", "zone", zoneName, "zone_id", z.ID, "subdomain", subDomain)
				return z.ID, subDomain, nil
			}
		}
	}

	return "", "", fmt.Errorf("zone not found for domain %s", domain)
}

// getZone 获取单个 zone 详情
func (c *Client) getZone(ctx context.Context, zoneID string) (*Zone, error) {
	url := fmt.Sprintf("%s/api/v2/zones/%s", c.baseURL, zoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zone request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get zone, status: %d", resp.StatusCode)
	}

	var zone Zone
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		return nil, fmt.Errorf("failed to decode zone response: %w", err)
	}
	return &zone, nil
}

// updateZoneIP 更新 zone 的 IPv6 地址（用于主域名）
func (c *Client) updateZoneIP(ctx context.Context, zoneID, ipv6 string) error {
	payload := map[string]string{"ipv6address": ipv6}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v2/zones/%s", c.baseURL, zoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Dynv6 API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Info("Dynv6 zone IPv6 updated", "zone_id", zoneID, "ipv6", ipv6)
	return nil
}

// setAuth 设置认证头
func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
}
