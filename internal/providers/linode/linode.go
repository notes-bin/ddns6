// Package linode 实现 Linode（Akamai）DNS API v4 服务。
//
// 认证方式：Personal Access Token。
// 必填参数：--api-key
//
// API 文档：https://www.linode.com/docs/api/domains/
package linode

import (
	"bytes"
	"cmp"
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
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

const defaultBaseURL = "https://api.linode.com/v4/domains"

// Client Linode DNS API 客户端。
type Client struct {
	apiKey  string
	baseURL string
	*http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 Linode DNS 客户端。
func NewClient(apiKey string, options ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		Client:  &http.Client{Timeout: 15 * time.Second},
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
		c.Client = httpClient
	}
}

type domain struct {
	ID     int    `json:"id"`
	Domain string `json:"domain"`
}

type domainRecord struct {
	ID     int    `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Target string `json:"target"`
	TTL    int    `json:"ttl_sec"`
}

type listResponse struct {
	Data []json.RawMessage `json:"data"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	domainID, sub, err := c.resolveDomain(ctx, record.Name, record.Zone)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"type":    record.Type,
		"name":    sub,
		"target":  record.Value,
		"ttl_sec": cmp.Or(record.TTL, ddns.DefaultTTL),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	slog.Debug("adding Linode DNS record", "module", "linode", "domain_id", domainID, "name", sub, "type", record.Type)
	_, err = c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/%d/records", domainID), payload)
	if err != nil {
		return err
	}
	slog.Info("Linode DNS record added", "module", "linode", "domain_id", domainID, "type", record.Type, "ipv6", record.Value)
	return nil
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	domainID, _, err := c.resolveDomain(ctx, record.Name, record.Zone)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"target":  record.Value,
		"ttl_sec": cmp.Or(record.TTL, ddns.DefaultTTL),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	slog.Debug("modifying Linode DNS record", "module", "linode", "record_id", record.ID)
	_, err = c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/%d/records/%s", domainID, record.ID), payload)
	return err
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	domainID, _, err := c.resolveDomain(ctx, record.Name, record.Zone)
	if err != nil {
		return err
	}

	slog.Debug("deleting Linode DNS record", "module", "linode", "record_id", record.ID)
	_, err = c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/%d/records/%s", domainID, record.ID), nil)
	return err
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	domainID, zone, err := c.findDomainID(ctx, fulldomain)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/%d/records", domainID), nil)
	if err != nil {
		return nil, err
	}

	var resp listResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		var records []domainRecord
		if err2 := json.Unmarshal(body, &records); err2 != nil {
			return nil, fmt.Errorf("failed to decode Linode records: %w", err)
		}
		return c.filterRecords(records, zone, recordType), nil
	}

	records := make([]domainRecord, 0, len(resp.Data))
	for _, raw := range resp.Data {
		var r domainRecord
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		records = append(records, r)
	}
	return c.filterRecords(records, zone, recordType), nil
}

func (c *Client) filterRecords(records []domainRecord, zone, recordType string) []ddns.RecordInfo {
	result := make([]ddns.RecordInfo, 0, len(records))
	for _, r := range records {
		if recordType != "" && r.Type != recordType {
			continue
		}
		name := zone
		if r.Name != "" && r.Name != "@" {
			name = r.Name + "." + zone
		}
		result = append(result, ddns.RecordInfo{
			ID:    fmt.Sprintf("%d", r.ID),
			Name:  name,
			Zone:  zone,
			Type:  r.Type,
			Value: r.Target,
			TTL:   r.TTL,
		})
	}
	return result
}

func (c *Client) resolveDomain(ctx context.Context, name, zone string) (domainID int, sub string, err error) {
	_, sub = domainutil.SplitDomain(name, zone)
	id, _, err := c.findDomainID(ctx, name)
	if err != nil {
		return 0, "", err
	}
	if sub == "" {
		sub = "@"
	}
	return id, sub, nil
}

func (c *Client) findDomainID(ctx context.Context, fulldomain string) (int, string, error) {
	parts := strings.Split(strings.TrimSuffix(fulldomain, "."), ".")
	for i := 1; i < len(parts); i++ {
		candidate := strings.Join(parts[i:], ".")
		filter := url.QueryEscape(fmt.Sprintf(`{"domain":"%s"}`, candidate))
		body, err := c.doRequestWithFilter(ctx, filter)
		if err != nil {
			return 0, "", err
		}

		var resp listResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			var domains []domain
			if err2 := json.Unmarshal(body, &domains); err2 != nil {
				return 0, "", fmt.Errorf("failed to decode Linode domains: %w", err)
			}
			for _, d := range domains {
				if strings.EqualFold(d.Domain, candidate) {
					return d.ID, candidate, nil
				}
			}
			continue
		}

		for _, raw := range resp.Data {
			var d domain
			if err := json.Unmarshal(raw, &d); err != nil {
				continue
			}
			if strings.EqualFold(d.Domain, candidate) {
				return d.ID, candidate, nil
			}
		}
	}
	return 0, "", fmt.Errorf("Linode domain not found for %s", fulldomain)
}

func (c *Client) doRequestWithFilter(ctx context.Context, filter string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?page_size=500", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Filter", filter)

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Linode API request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(body)}
	}
	return body, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error
	endpoint := c.baseURL + path
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, endpoint, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Linode API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Linode response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(respBody)}
	}
	return respBody, nil
}

// httpStatusError 表示 Linode API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("Linode API error: status %d, body: %s", e.status, e.body)
}
