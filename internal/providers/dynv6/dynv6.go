// Package dynv6 实现 Dynv6 免费 DDNS 服务。
//
// 认证方式：API Token（Bearer Token）。
// 必填参数：--token。
// 使用 RESTful JSON API；支持 zone 自动发现与 CRUD；路径段经 url.PathEscape 转义。
package dynv6

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/internal/httputil"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

// 编译期断言：Client 实现 ddns.DNSProvider。
var _ ddns.DNSProvider = (*Client)(nil)

const (
	defaultBaseURL = "https://dynv6.com"
)

// Client Dynv6 API 客户端。
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
	zoneCache  sync.Map // zone 名称 -> zone ID
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 Dynv6 客户端。
// 默认使用 httputil.NewHTTPClient（超时 + 同主机重定向限制）。
func NewClient(token string, options ...Option) *Client {
	c := &Client{
		token:      token,
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

// zone Dynv6 区域信息。
type zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IPv4 string `json:"ipv4address"`
	IPv6 string `json:"ipv6address"`
}

// dnsRecord Dynv6 DNS 记录。
type dnsRecord struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Data string `json:"data"`
	TTL  int    `json:"ttl,omitzero"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, subDomain, err := c.resolveZone(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to resolve zone: %w", err)
	}

	// 主域名（无子域名）直接更新 zone 的 IPv6 地址
	if subDomain == "" || subDomain == "@" {
		return c.updateZoneIP(ctx, zoneID, record.Value)
	}

	dnsRec := dnsRecord{
		Type: record.Type,
		Name: subDomain,
		Data: record.Value,
	}
	body, err := json.Marshal(dnsRec)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/v2/zones/%s/records", c.baseURL, url.PathEscape(zoneID))
	slog.Debug("adding Dynv6 record", "module", "dynv6", "zone_id", zoneID, "name", subDomain, "type", record.Type)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dynv6 api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := httputil.ReadBody(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read error response body: %w", readErr)
		}
		return fmt.Errorf("dynv6 api error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Info("Dynv6 record added successfully", "module", "dynv6", "zone_id", zoneID, "name", subDomain, "ipv6", record.Value)
	return nil
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, _, err := c.resolveZone(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to resolve zone: %w", err)
	}

	dnsRec := dnsRecord{
		Type: record.Type,
		Data: record.Value,
		TTL:  ddns.RecordTTL(record.TTL),
	}
	body, err := json.Marshal(dnsRec)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/v2/zones/%s/records/%s", c.baseURL, url.PathEscape(zoneID), url.PathEscape(record.ID))
	slog.Debug("modifying Dynv6 record", "module", "dynv6", "zone_id", zoneID, "record_id", record.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dynv6 api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := httputil.ReadBody(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read error response body: %w", readErr)
		}
		return fmt.Errorf("dynv6 api error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Info("Dynv6 record modified successfully", "module", "dynv6", "zone_id", zoneID, "record_id", record.ID, "ipv6", record.Value)
	return nil
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, _, err := c.resolveZone(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to resolve zone: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/v2/zones/%s/records/%s", c.baseURL, url.PathEscape(zoneID), url.PathEscape(record.ID))
	slog.Debug("deleting Dynv6 record", "module", "dynv6", "zone_id", zoneID, "record_id", record.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dynv6 api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := httputil.ReadBody(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read error response body: %w", readErr)
		}
		return fmt.Errorf("dynv6 api error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Info("Dynv6 record deleted successfully", "module", "dynv6", "zone_id", zoneID, "record_id", record.ID)
	return nil
}

// GetRecords 查询 DNS 记录（合并 zone 级 IPv6 与 zone 下 records 列表，再按类型过滤）。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zoneID, _, err := c.resolveZone(ctx, fulldomain, "")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve zone: %w", err)
	}

	result := make([]ddns.RecordInfo, 0)

	zone, err := c.getZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	// zone 级 IPv6（apex AAAA）
	if zone.IPv6 != "" && (recordType == "" || recordType == "AAAA") {
		result = append(result, ddns.RecordInfo{
			ID: zoneID, Name: zone.Name, Type: "AAAA", Value: zone.IPv6,
		})
	}

	// 子域名 records（始终拉取，供编排层匹配 www 等）
	reqURL := fmt.Sprintf("%s/api/v2/zones/%s/records", c.baseURL, url.PathEscape(zoneID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, httputil.WrapRequestError("dynv6 api request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := httputil.ReadBody(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read error response body: %w", readErr)
		}
		return nil, fmt.Errorf("dynv6 api error: status %d, body: %s", resp.StatusCode, httputil.TruncateForLog(string(bodyBytes)))
	}

	var records []dnsRecord
	if err := json.NewDecoder(httputil.LimitBody(resp.Body)).Decode(&records); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, r := range records {
		if recordType != "" && r.Type != recordType {
			continue
		}
		name := zone.Name
		if r.Name != "" && r.Name != "@" {
			name = r.Name + "." + zone.Name
		}
		result = append(result, ddns.RecordInfo{
			ID:    r.ID,
			Name:  name,
			Type:  r.Type,
			Value: r.Data,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// resolveZone 解析域名对应的 zone ID 和子域名。
// 有 zoneHint 时优先按缓存/精确名匹配；否则 list zones 后按后缀匹配。
func (c *Client) resolveZone(ctx context.Context, domain, zoneHint string) (string, string, error) {
	root, sub := domainutil.SplitDomain(domain, zoneHint)
	if root == "" {
		root = domain
		sub = ""
	}
	if sub == "@" {
		sub = ""
	}

	if v, ok := c.zoneCache.Load(root); ok {
		if id, ok := v.(string); ok {
			return id, sub, nil
		}
	}

	url := c.baseURL + "/api/v2/zones"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to list zones: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to list zones, status: %d", resp.StatusCode)
	}

	var zones []zone
	if err := json.NewDecoder(httputil.LimitBody(resp.Body)).Decode(&zones); err != nil {
		return "", "", fmt.Errorf("failed to decode zones: %w", err)
	}

	// 先精确匹配 root，再后缀回退
	for _, z := range zones {
		c.zoneCache.Store(z.Name, z.ID)
	}
	if v, ok := c.zoneCache.Load(root); ok {
		if id, ok := v.(string); ok {
			slog.Debug("resolved Dynv6 zone", "module", "dynv6", "zone", root, "zone_id", id, "subdomain", sub)
			return id, sub, nil
		}
	}

	parts := strings.Split(domain, ".")
	for i := range len(parts) {
		zoneName := strings.Join(parts[i:], ".")
		if v, ok := c.zoneCache.Load(zoneName); ok {
			id, ok := v.(string)
			if !ok {
				continue
			}
			subDomain := ""
			if i > 0 {
				subDomain = strings.Join(parts[:i], ".")
			}
			slog.Debug("resolved Dynv6 zone", "module", "dynv6", "zone", zoneName, "zone_id", id, "subdomain", subDomain)
			return id, subDomain, nil
		}
	}

	return "", "", fmt.Errorf("zone not found for domain %s", domain)
}

// getZone 获取单个 zone 详情。
func (c *Client) getZone(ctx context.Context, zoneID string) (*zone, error) {
	reqURL := fmt.Sprintf("%s/api/v2/zones/%s", c.baseURL, url.PathEscape(zoneID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zone request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get zone, status: %d", resp.StatusCode)
	}

	var zone zone
	if err := json.NewDecoder(httputil.LimitBody(resp.Body)).Decode(&zone); err != nil {
		return nil, fmt.Errorf("failed to decode zone response: %w", err)
	}
	return &zone, nil
}

// updateZoneIP 更新 zone 的 IPv6 地址（用于主域名）。
func (c *Client) updateZoneIP(ctx context.Context, zoneID, ipv6 string) error {
	payload := map[string]string{"ipv6address": ipv6}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/api/v2/zones/%s", c.baseURL, url.PathEscape(zoneID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := httputil.ReadBody(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read error response body: %w", readErr)
		}
		return fmt.Errorf("dynv6 api error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Info("Dynv6 zone IPv6 updated", "module", "dynv6", "zone_id", zoneID, "ipv6", ipv6)
	return nil
}

// setAuth 设置 Bearer Token 认证头。
func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
}
