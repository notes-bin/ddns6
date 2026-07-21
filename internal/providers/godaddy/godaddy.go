package godaddy

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
)

// GoDaddyClient GoDaddy DNS API 客户端
type GoDaddyClient struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	HTTPClient *http.Client
}

type Options func(*GoDaddyClient)

// NewClient 创建 GoDaddy DNS 客户端
func NewClient(apiKey, apiSecret string, options ...Options) *GoDaddyClient {
	client := &GoDaddyClient{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    "https://api.godaddy.com/v1",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL 设置自定义 API 地址（测试用）
func WithBaseURL(baseURL string) Options {
	return func(c *GoDaddyClient) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(httpClient *http.Client) Options {
	return func(c *GoDaddyClient) {
		c.HTTPClient = httpClient
	}
}

// DNSRecord  a GoDaddy DNS record
type DNSRecord struct {
	Data string `json:"data"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	TTL  int    `json:"ttl,omitempty"`
}

// AddRecord 添加域名解析记录
func (c *GoDaddyClient) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	subDomain, domain, err := c.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	existingRecords, err := c.getRecords(ctx, domain, subDomain, record.Type)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	for _, r := range existingRecords {
		if r.Data == record.Value {
			return nil
		}
	}

	newRecord := DNSRecord{
		Data: record.Value,
		Type: record.Type,
		Name: subDomain,
		TTL:  record.TTL,
	}
	newRecords := append(existingRecords, newRecord)

	return c.updateRecords(ctx, domain, subDomain, record.Type, newRecords)
}

// ModifyRecord 修改域名解析记录
func (c *GoDaddyClient) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	subDomain, domain, err := c.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	existingRecords, err := c.getRecords(ctx, domain, subDomain, record.Type)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	// record.ID is used as the old value to match for GoDaddy (no record ID concept)
	var modified bool
	for i, r := range existingRecords {
		if r.Data == record.ID {
			existingRecords[i].Data = record.Value
			existingRecords[i].TTL = record.TTL
			modified = true
			break
		}
	}

	if !modified {
		return fmt.Errorf("record not found")
	}

	return c.updateRecords(ctx, domain, subDomain, record.Type, existingRecords)
}

// DeleteRecord 删除域名解析记录，record.ID 作为匹配值（GoDaddy 无 ID 概念）
func (c *GoDaddyClient) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	subDomain, domain, err := c.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	// 按值匹配删除 AAAA 记录
	return c.deleteRecordsByValue(ctx, domain, subDomain, "AAAA", record.ID)
}

// deleteRecordsByValue 根据值删除特定类型的记录
func (c *GoDaddyClient) deleteRecordsByValue(ctx context.Context, domain, subDomain, rtype, value string) error {
	existingRecords, err := c.getRecords(ctx, domain, subDomain, rtype)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	var newRecords []DNSRecord
	var found bool
	for _, record := range existingRecords {
		if record.Data != value {
			newRecords = append(newRecords, record)
		} else {
			found = true
		}
	}

	if !found {
		return nil
	}

	if len(newRecords) == 0 {
		return c.deleteRecords(ctx, domain, subDomain, rtype)
	}

	return c.updateRecords(ctx, domain, subDomain, rtype, newRecords)
}

// GetRecords 查询域名的解析记录，返回通用 RecordInfo 列表
func (c *GoDaddyClient) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	subDomain, domain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}

	records, err := c.getRecords(ctx, domain, subDomain, recordType)
	if err != nil {
		return nil, err
	}

	result := make([]ddns.RecordInfo, len(records))
	for i, r := range records {
		result[i] = ddns.RecordInfo{
			ID:    r.Data, // GoDaddy 无 ID 概念，用值作为标识
			Name:  r.Name,
			Type:  r.Type,
			Value: r.Data,
			TTL:   r.TTL,
		}
	}
	return result, nil
}

// getRecords 获取指定类型的记录
func (c *GoDaddyClient) getRecords(ctx context.Context, domain, subDomain, rtype string) ([]DNSRecord, error) {
	// subDomain 为 "@" 时表示根域名，不传入 name 路径段以获取该域名下所有记录
	url := fmt.Sprintf("%s/domains/%s/records/%s", c.BaseURL, domain, rtype)
	if subDomain != "@" {
		url += "/" + subDomain
	}
	var records []DNSRecord
	err := c.makeRequest(ctx, "GET", url, nil, &records)
	return records, err
}

// updateRecords 更新记录
func (c *GoDaddyClient) updateRecords(ctx context.Context, domain, subDomain, rtype string, records []DNSRecord) error {
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", c.BaseURL, domain, rtype, subDomain)
	body, err := json.Marshal(records)
	if err != nil {
		return err
	}
	return c.makeRequest(ctx, "PUT", url, bytes.NewBuffer(body), nil)
}

// deleteRecords 删除所有记录
func (c *GoDaddyClient) deleteRecords(ctx context.Context, domain, subDomain, rtype string) error {
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", c.BaseURL, domain, rtype, subDomain)
	return c.makeRequest(ctx, "DELETE", url, nil, nil)
}

// getRootDomain finds the root domain and subdomain
func (c *GoDaddyClient) getRootDomain(ctx context.Context, domain string) (string, string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")
		slog.Debug("probing GoDaddy root domain", "module", "godaddy", "domain", h)

		_, err := c.getDomain(ctx, h)
		if err == nil {
			subDomain := strings.Join(parts[:i], ".")
			slog.Info("GoDaddy root domain found", "module", "godaddy", "root", h, "subdomain", subDomain)
			return subDomain, h, nil
		}
	}

	// 兜底：完整域名即为根域名，使用 @ 表示 apex 记录
	_, err := c.getDomain(ctx, domain)
	if err == nil {
		return "@", domain, nil
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// getDomain checks if a domain exists in GoDaddy
func (c *GoDaddyClient) getDomain(ctx context.Context, domain string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/domains/%s", c.BaseURL, domain)
	var result map[string]interface{}
	err := c.makeRequest(ctx, "GET", url, nil, &result)
	return result, err
}

// makeRequest performs an HTTP request to the GoDaddy API
func (c *GoDaddyClient) makeRequest(ctx context.Context, method, url string, body io.Reader, result interface{}) error {
	slog.Debug("GoDaddy API request", "module", "godaddy", "method", method, "url", url)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", c.APIKey, c.APISecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		slog.Error("GoDaddy API request failed", "module", "godaddy", "method", method, "url", url, "err", err)
		return err
	}
	defer resp.Body.Close()

	slog.Debug("GoDaddy API response", "module", "godaddy", "method", method, "status", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("GoDaddy API returned error status",
			"module", "godaddy",
			"method", method, "status", resp.StatusCode)
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return err
		}
	}

	return nil
}
