// Package huaweicloud 实现华为云 DNS API 服务
//
// 认证方式：Access Key + Secret Key（从 IAM 用户获取）
// 必填参数：--access-key, --secret-key
//
// 使用 SDK-HMAC-SHA256 签名认证，基于 AWS SigV4 变体
// API 文档：https://support.huaweicloud.com/api-dns/dns_api_64001.html
package huaweicloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
)

const (
	defaultBaseURL = "https://dns.myhuaweicloud.com"
)

// Client 华为云 DNS API 客户端
type Client struct {
	accessKey string
	secretKey string
	baseURL   string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建华为云 DNS 客户端
func NewClient(accessKey, secretKey string, options ...Option) *Client {
	c := &Client{
		accessKey: accessKey,
		secretKey: secretKey,
		baseURL:   defaultBaseURL,
		Client:    &http.Client{Timeout: 30 * time.Second},
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

// DNSRecord 华为云 DNS 记录集
type DNSRecord struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Records []string `json:"records"`
	Weight  *int     `json:"weight,omitempty"`
	ZoneID  string   `json:"zone_id,omitempty"`
}

// recordSetPayload 创建记录集时的请求体
// weight 只在创建时需要，更新时不传
type recordSetPayload struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Records []string `json:"records"`
	Weight  int      `json:"weight"`
}

// AddRecord 添加域名解析记录
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, err := c.getZoneID(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %w", err)
	}

	payload := recordSetPayload{
		Name:    record.Name + ".",
		Type:    record.Type,
		TTL:     record.TTL,
		Records: []string{record.Value},
		Weight:  1,
	}

	url := c.baseURL + "/v2.1/zones/" + zoneID + "/recordsets"
	slog.Debug("adding HuaweiCloud DNS record", "module", "huaweicloud", "zone", zoneID, "domain", record.Name, "type", record.Type)

	_, err = c.request(ctx, http.MethodPost, url, payload)
	if err != nil {
		return err
	}

	slog.Info("HuaweiCloud DNS record added successfully", "module", "huaweicloud", "zone", zoneID, "name", record.Name, "ipv6", record.Value)
	return nil
}

// ModifyRecord 修改域名解析记录
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, err := c.getZoneID(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %w", err)
	}

	payload := map[string]any{
		"name":    record.Name + ".",
		"type":    record.Type,
		"ttl":     record.TTL,
		"records": []string{record.Value},
	}

	url := c.baseURL + "/v2.1/zones/" + zoneID + "/recordsets/" + record.ID
	slog.Debug("modifying HuaweiCloud DNS record", "module", "huaweicloud", "zone", zoneID, "record_id", record.ID)

	_, err = c.request(ctx, http.MethodPut, url, payload)
	if err != nil {
		return err
	}

	slog.Info("HuaweiCloud DNS record modified successfully", "module", "huaweicloud", "zone", zoneID, "record_id", record.ID, "ipv6", record.Value)
	return nil
}

// DeleteRecord 删除域名解析记录
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, err := c.getZoneID(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %w", err)
	}

	url := c.baseURL + "/v2.1/zones/" + zoneID + "/recordsets/" + record.ID
	slog.Debug("deleting HuaweiCloud DNS record", "module", "huaweicloud", "zone", zoneID, "record_id", record.ID)

	_, err = c.request(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	slog.Info("HuaweiCloud DNS record deleted successfully", "module", "huaweicloud", "zone", zoneID, "record_id", record.ID)
	return nil
}

// GetRecords 查询域名的解析记录，返回通用 RecordInfo 列表
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zoneID, err := c.getZoneID(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %w", err)
	}

	// 查询租户下指定 zone 的记录集列表
	params := url.Values{}
	params.Set("type", recordType)
	params.Set("name", fulldomain+".")
	params.Set("limit", "500")

	// 华为云 API 分页获取全部 recordsets
	var allRecordsets []DNSRecord
	marker := ""
	for {
		if marker != "" {
			params.Set("marker", marker)
		}
		reqURL := c.baseURL + "/v2.1/zones/" + zoneID + "/recordsets?" + params.Encode()

		var apiResult struct {
			Recordsets []DNSRecord `json:"recordsets"`
			Links      struct {
				Next string `json:"next"`
			} `json:"links"`
		}

		if err := c.requestRaw(ctx, http.MethodGet, reqURL, &apiResult); err != nil {
			return nil, err
		}

		allRecordsets = append(allRecordsets, apiResult.Recordsets...)

		if apiResult.Links.Next == "" {
			break
		}
		// 从 next link 中提取 marker
		if u, parseErr := url.Parse(apiResult.Links.Next); parseErr == nil {
			marker = u.Query().Get("marker")
		}
		if marker == "" {
			break
		}
	}

	records := make([]ddns.RecordInfo, 0, len(allRecordsets))
	for _, r := range allRecordsets {
		value := ""
		if len(r.Records) > 0 {
			value = r.Records[0]
		}
		records = append(records, ddns.RecordInfo{
			ID:    r.ID,
			Name:  r.Name,
			Type:  r.Type,
			Value: value,
			TTL:   r.TTL,
		})
	}
	return records, nil
}

// getZoneID 查找域名对应的 Zone ID
func (c *Client) getZoneID(ctx context.Context, domain string) (string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")
		slog.Debug("looking up HuaweiCloud zone", "module", "huaweicloud", "domain", h)

		reqURL := c.baseURL + "/v2/zones?name=" + url.QueryEscape(h)

		var result struct {
			Zones []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"zones"`
		}

		if err := c.requestRaw(ctx, http.MethodGet, reqURL, &result); err != nil {
			continue
		}

		for _, zone := range result.Zones {
			if zone.Name == h+"." {
				slog.Info("HuaweiCloud zone found",
					"module", "huaweicloud",
					"zone", h, "zone_id", zone.ID)
				return zone.ID, nil
			}
		}
	}

	// 兜底：完整域名即为区域
	reqURL := c.baseURL + "/v2/zones?name=" + url.QueryEscape(domain)
	var result struct {
		Zones []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"zones"`
	}
	if err := c.requestRaw(ctx, http.MethodGet, reqURL, &result); err == nil {
		for _, zone := range result.Zones {
			if zone.Name == domain+"." {
				slog.Info("HuaweiCloud zone found (fallback)",
					"module", "huaweicloud",
					"zone", domain, "zone_id", zone.ID)
				return zone.ID, nil
			}
		}
	}

	return "", fmt.Errorf("zone not found for domain %s", domain)
}

// request 执行签名 HTTP 请求，自动解码响应
func (c *Client) request(ctx context.Context, method, url string, payload any) ([]byte, error) {
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// SDK-HMAC-SHA256 签名
	signer := &Signer{Key: c.accessKey, Secret: c.secretKey}
	if err := signer.Sign(req); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HuaweiCloud API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HuaweiCloud API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// requestRaw 执行签名 HTTP 请求并解码到目标结构体（用于 GET 请求）
func (c *Client) requestRaw(ctx context.Context, method, url string, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// SDK-HMAC-SHA256 签名
	signer := &Signer{Key: c.accessKey, Secret: c.secretKey}
	if err := signer.Sign(req); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("HuaweiCloud API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HuaweiCloud API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}
