// Package cloudflare 实现 Cloudflare DNS API 服务。
//
// 认证方式：API Token（需具有 DNS:Edit 权限）
// 必填参数：--api-token
//
// 使用 RESTful JSON API，支持 Zone ID 自动发现。
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/internal/httputil"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

// 编译期断言：Client 实现 ddns.DNSProvider。
var _ ddns.DNSProvider = (*Client)(nil)

// Client Cloudflare DNS API 客户端。
type Client struct {
	apiKey     string
	email      string
	apiToken   string
	accountID  string
	zoneID     string
	baseURL    string
	httpClient *http.Client
	zoneCache  sync.Map // zone 名称 -> zone ID
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 Cloudflare DNS 客户端。
func NewClient(options ...Option) *Client {
	client := &Client{
		baseURL:    "https://api.cloudflare.com/client/v4",
		httpClient: httputil.NewHTTPClient(30 * time.Second),
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithAPIKey 设置 API Key 与 email（旧版认证方式）。
func WithAPIKey(apiKey, email string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
		c.email = email
	}
}

// WithAPIToken 设置 API Token（推荐认证方式）。
func WithAPIToken(apiToken string) Option {
	return func(c *Client) {
		c.apiToken = apiToken
	}
}

// WithAccountID 设置账户 ID。
func WithAccountID(accountID string) Option {
	return func(c *Client) {
		c.accountID = accountID
	}
}

// WithZoneID 设置 Zone ID。
func WithZoneID(zoneID string) Option {
	return func(c *Client) {
		c.zoneID = zoneID
	}
}

// WithBaseURL 设置自定义 API 基址（测试用）。
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

// dnsRecord 表示 Cloudflare DNS 记录。
type dnsRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl,omitzero"`
}

// APIResponse 表示 Cloudflare API 标准响应。
type APIResponse struct {
	Success  bool            `json:"success"`
	Errors   []ErrorDetails  `json:"errors"`
	Messages []string        `json:"messages"`
	Result   json.RawMessage `json:"result"`
}

// ErrorDetails 表示 Cloudflare API 错误详情。
type ErrorDetails struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, err := c.getZoneID(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to get zone id: %w", err)
	}

	records, err := c.getRecords(ctx, zoneID, record.Name, record.Type, record.Value)
	if err != nil {
		return fmt.Errorf("failed to check existing records: %w", err)
	}

	if len(records) > 0 {
		return nil
	}

	cfRecord := dnsRecord{
		Type:    record.Type,
		Name:    record.Name,
		Content: record.Value,
		TTL:     ddns.RecordTTL(record.TTL),
	}

	_, err = c.createDNSRecord(ctx, zoneID, cfRecord)
	return err
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, err := c.getZoneID(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to get zone id: %w", err)
	}

	cfRecord, err := c.getRecordByID(ctx, zoneID, record.ID)
	if err != nil {
		return fmt.Errorf("failed to get record: %w", err)
	}

	cfRecord.Content = record.Value
	cfRecord.TTL = ddns.RecordTTL(record.TTL)

	_, err = c.updateDNSRecord(ctx, zoneID, cfRecord.ID, *cfRecord)
	return err
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, err := c.getZoneID(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to get zone id: %w", err)
	}

	return c.deleteDNSRecord(ctx, zoneID, record.ID)
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zoneID, err := c.getZoneID(ctx, fulldomain, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get zone id: %w", err)
	}

	records, err := c.getRecords(ctx, zoneID, "", recordType, "")
	if err != nil {
		return nil, err
	}

	result := make([]ddns.RecordInfo, len(records))
	for i, r := range records {
		result[i] = ddns.RecordInfo{
			ID:    r.ID,
			Name:  r.Name,
			Type:  r.Type,
			Value: r.Content,
			TTL:   r.TTL,
		}
	}
	return result, nil
}

// getDomainRecord 查询单条 DNS 记录详情。
func (c *Client) getDomainRecord(ctx context.Context, fulldomain, recordID string) (*dnsRecord, error) {
	zoneID, err := c.getZoneID(ctx, fulldomain, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get zone id: %w", err)
	}

	return c.getRecordByID(ctx, zoneID, recordID)
}

// resultInfo 表示 Cloudflare API 分页信息。
type resultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
	TotalCount int `json:"total_count"`
}

// listDNSRecords 分页获取指定类型和名称的 DNS 记录（公共分页逻辑）。
func (c *Client) listDNSRecords(ctx context.Context, zoneID, name, rtype, content string) ([]dnsRecord, *resultInfo, error) {
	var allRecords []dnsRecord
	var lastInfo *resultInfo
	page := 1

	for {
		query := url.Values{}
		query.Set("type", rtype)
		if name != "" {
			query.Set("name", name)
		}
		query.Set("per_page", "100")
		query.Set("page", strconv.Itoa(page))
		if content != "" {
			query.Set("content", content)
		}
		reqURL := fmt.Sprintf("%s/zones/%s/dns_records?%s", c.baseURL, url.PathEscape(zoneID), query.Encode())

		records, info, err := c.listRequest(ctx, reqURL)
		if err != nil {
			return nil, nil, err
		}
		allRecords = append(allRecords, records...)
		lastInfo = info

		if info == nil || page >= info.TotalPages {
			break
		}
		page++
	}

	return allRecords, lastInfo, nil
}

// getRecords 获取指定类型的 DNS 记录。
func (c *Client) getRecords(ctx context.Context, zoneID, name, rtype, content string) ([]dnsRecord, error) {
	records, _, err := c.listDNSRecords(ctx, zoneID, name, rtype, content)
	return records, err
}

// listRequest 执行 GET 请求并返回记录与分页信息。
func (c *Client) listRequest(ctx context.Context, reqURL string) ([]dnsRecord, *resultInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	} else {
		req.Header.Set("X-Auth-Email", c.email)
		req.Header.Set("X-Auth-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiResp APIResponse
		if err := json.NewDecoder(httputil.LimitBody(resp.Body)).Decode(&apiResp); err == nil {
			if len(apiResp.Errors) > 0 {
				return nil, nil, fmt.Errorf("cloudflare api error: %s (code %d)",
					apiResp.Errors[0].Message, apiResp.Errors[0].Code)
			}
		}
		return nil, nil, fmt.Errorf("http request failed with status %d", resp.StatusCode)
	}

	var apiResp struct {
		Success    bool           `json:"success"`
		Errors     []ErrorDetails `json:"errors"`
		Result     []dnsRecord    `json:"result"`
		ResultInfo *resultInfo    `json:"result_info"`
	}

	if err := json.NewDecoder(httputil.LimitBody(resp.Body)).Decode(&apiResp); err != nil {
		return nil, nil, err
	}

	if !apiResp.Success {
		if len(apiResp.Errors) > 0 {
			return nil, nil, fmt.Errorf("cloudflare api error: %s", apiResp.Errors[0].Message)
		}
		return nil, nil, fmt.Errorf("cloudflare api request was not successful")
	}

	return apiResp.Result, apiResp.ResultInfo, nil
}

// getRecordByID 根据记录 ID 获取 DNS 记录。
func (c *Client) getRecordByID(ctx context.Context, zoneID, recordID string) (*dnsRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.baseURL, url.PathEscape(zoneID), url.PathEscape(recordID))
	var result dnsRecord
	err := c.makeRequest(ctx, "GET", url, nil, &result)
	return &result, err
}

// updateDNSRecord 更新 DNS 记录。
func (c *Client) updateDNSRecord(ctx context.Context, zoneID, recordID string, record dnsRecord) (*dnsRecord, error) {
	slog.Info("updating Cloudflare DNS record",
		"module", "cloudflare",
		"record_id", recordID, "type", record.Type, "zone_id", zoneID)

	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.baseURL, url.PathEscape(zoneID), url.PathEscape(recordID))

	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	var result dnsRecord
	err = c.makeRequest(ctx, "PUT", url, bytes.NewBuffer(body), &result)
	if err != nil {
		slog.Debug("failed to update Cloudflare DNS record",
			"module", "cloudflare",
			"record_id", recordID, "err", err)
	}
	return &result, err
}

// getZoneID 解析域名对应的 Zone ID。
//
// 优先顺序：配置的 ZoneID > zoneHint 缓存/直查 > 对 domain 后缀探测。
// 配置了 ZoneID 时直接返回，不再每次校验详情（避免多余 API）。
func (c *Client) getZoneID(ctx context.Context, domain, zoneHint string) (string, error) {
	if c.zoneID != "" {
		return c.zoneID, nil
	}

	root, _ := domainutil.SplitDomain(domain, zoneHint)
	if root == "" {
		root = domain
	}

	if v, ok := c.zoneCache.Load(root); ok {
		if id, ok := v.(string); ok {
			return id, nil
		}
	}

	// 已知根域名时直接按名查找，避免逐级后缀探测
	if zoneHint != "" || root != domain {
		if zoneID, err := c.findZoneID(ctx, root); err == nil {
			c.zoneCache.Store(root, zoneID)
			return zoneID, nil
		}
	}

	parts := strings.Split(domain, ".")
	for i := range len(parts) - 1 {
		zone := strings.Join(parts[i+1:], ".")
		zoneID, err := c.findZoneID(ctx, zone)
		if err == nil {
			c.zoneCache.Store(zone, zoneID)
			c.zoneCache.Store(root, zoneID)
			return zoneID, nil
		}
	}

	zoneID, err := c.findZoneID(ctx, domain)
	if err == nil {
		c.zoneCache.Store(domain, zoneID)
		return zoneID, nil
	}

	return "", fmt.Errorf("could not find zone id for domain %s", domain)
}

// findZoneID 按名称查找 Zone ID。
func (c *Client) findZoneID(ctx context.Context, zone string) (string, error) {
	slog.Debug("looking up Cloudflare zone", "module", "cloudflare", "zone", zone)

	query := url.Values{}
	query.Set("name", zone)
	query.Set("status", "active")
	if c.accountID != "" {
		query.Set("account.id", c.accountID)
	}
	reqURL := fmt.Sprintf("%s/zones?%s", c.baseURL, query.Encode())

	var result []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	err := c.makeRequest(ctx, "GET", reqURL, nil, &result)
	if err != nil {
		return "", err
	}

	for _, z := range result {
		if z.Name == zone {
			slog.Info("Cloudflare zone found", "module", "cloudflare", "zone", zone, "zone_id", z.ID)
			return z.ID, nil
		}
	}

	return "", fmt.Errorf("zone not found")
}

// createDNSRecord 创建 DNS 记录。
func (c *Client) createDNSRecord(ctx context.Context, zoneID string, record dnsRecord) (*dnsRecord, error) {
	slog.Info("creating Cloudflare DNS record",
		"module", "cloudflare",
		"type", record.Type, "name", record.Name, "zone_id", zoneID)

	url := fmt.Sprintf("%s/zones/%s/dns_records", c.baseURL, url.PathEscape(zoneID))

	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	var result dnsRecord
	err = c.makeRequest(ctx, "POST", url, bytes.NewBuffer(body), &result)
	if err != nil {
		slog.Debug("failed to create Cloudflare DNS record",
			"module", "cloudflare",
			"type", record.Type, "name", record.Name, "err", err)
	}
	return &result, err
}

// deleteDNSRecord 删除 DNS 记录。
func (c *Client) deleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	slog.Info("deleting Cloudflare DNS record", "module", "cloudflare", "record_id", recordID, "zone_id", zoneID)

	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.baseURL, url.PathEscape(zoneID), url.PathEscape(recordID))
	err := c.makeRequest(ctx, "DELETE", url, nil, nil)
	if err != nil {
		slog.Debug("failed to delete Cloudflare DNS record", "module", "cloudflare", "record_id", recordID, "err", err)
	}
	return err
}

// makeRequest 向 Cloudflare API 发送 HTTP 请求。
func (c *Client) makeRequest(ctx context.Context, method, url string, body io.Reader, result any) error {
	slog.Debug("Cloudflare API request", "module", "cloudflare", "method", method, "url", url)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	} else {
		req.Header.Set("X-Auth-Email", c.email)
		req.Header.Set("X-Auth-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Debug("Cloudflare API request failed", "module", "cloudflare", "method", method, "url", url, "err", err)
		return err
	}
	defer resp.Body.Close()

	slog.Debug("Cloudflare API response", "module", "cloudflare", "method", method, "status", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiResp APIResponse
		if err := json.NewDecoder(httputil.LimitBody(resp.Body)).Decode(&apiResp); err == nil {
			if len(apiResp.Errors) > 0 {
				slog.Debug("Cloudflare API returned error",
					"module", "cloudflare",
					"method", method, "status", resp.StatusCode,
					"code", apiResp.Errors[0].Code,
					"message", apiResp.Errors[0].Message)
				return fmt.Errorf("cloudflare api error: %s (code %d)",
					apiResp.Errors[0].Message, apiResp.Errors[0].Code)
			}
		}
		return fmt.Errorf("http request failed with status %d", resp.StatusCode)
	}

	if result != nil {
		var apiResp APIResponse
		if err := json.NewDecoder(httputil.LimitBody(resp.Body)).Decode(&apiResp); err != nil {
			return err
		}

		if !apiResp.Success {
			if len(apiResp.Errors) > 0 {
				slog.Debug("Cloudflare API operation failed",
					"module", "cloudflare",
					"method", method, "message", apiResp.Errors[0].Message)
				return fmt.Errorf("cloudflare api error: %s", apiResp.Errors[0].Message)
			}
			return fmt.Errorf("cloudflare api request was not successful")
		}

		if apiResp.Result != nil {
			if err := json.Unmarshal(apiResp.Result, result); err != nil {
				return err
			}
		}
	}

	return nil
}
