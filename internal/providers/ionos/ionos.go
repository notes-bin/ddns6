// Package ionos 实现 IONOS DNS API 服务。
//
// 认证方式：API Key（由 prefix 与 secret 组成）。
// 必填参数：--prefix、--secret
//
// API 文档：https://developer.hosting.ionos.com/docs/dns
package ionos

import (
	"bytes"
	"cmp"
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

const defaultBaseURL = "https://api.hosting.ionos.com/dns/v1"

// Client IONOS DNS API 客户端。
type Client struct {
	prefix     string
	secret     string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 IONOS DNS 客户端。
func NewClient(prefix, secret string, options ...Option) *Client {
	c := &Client{
		prefix:     prefix,
		secret:     secret,
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

type zone struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Records []record `json:"records"`
}

type record struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Prio     int    `json:"prio"`
	Disabled bool   `json:"disabled"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, info ddns.RecordInfo) error {
	zoneID, fqdn, err := c.resolveZone(ctx, info.Name, info.Zone)
	if err != nil {
		return err
	}

	payload, err := json.Marshal([]record{{
		Name:     fqdn,
		Type:     info.Type,
		Content:  info.Value,
		TTL:      max(cmp.Or(info.TTL, ddns.DefaultTTL), 60),
		Prio:     0,
		Disabled: false,
	}})
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	slog.Debug("adding IONOS DNS record", "module", "ionos", "zone_id", zoneID, "name", fqdn, "type", info.Type)
	_, err = c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/records", zoneID), payload)
	if err != nil {
		return err
	}
	slog.Info("IONOS DNS record added", "module", "ionos", "name", fqdn, "type", info.Type, "ipv6", info.Value)
	return nil
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, info ddns.RecordInfo) error {
	zoneID, fqdn, err := c.resolveZone(ctx, info.Name, info.Zone)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(record{
		Name:     fqdn,
		Type:     info.Type,
		Content:  info.Value,
		TTL:      max(cmp.Or(info.TTL, ddns.DefaultTTL), 60),
		Prio:     0,
		Disabled: false,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	_, err = c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/zones/%s/records/%s", zoneID, info.ID), payload)
	return err
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, info ddns.RecordInfo) error {
	zoneID, _, err := c.resolveZone(ctx, info.Name, info.Zone)
	if err != nil {
		return err
	}
	_, err = c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s/records/%s", zoneID, info.ID), nil)
	return err
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zoneID, zoneName, err := c.findZone(ctx, fulldomain)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/zones/%s", zoneID)
	if recordType != "" {
		path += "?recordType=" + recordType
	}
	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var z zone
	if err := json.Unmarshal(body, &z); err != nil {
		return nil, fmt.Errorf("failed to decode IONOS zone: %w", err)
	}

	result := make([]ddns.RecordInfo, 0, len(z.Records))
	for _, r := range z.Records {
		if recordType != "" && r.Type != recordType {
			continue
		}
		result = append(result, ddns.RecordInfo{
			ID:    r.ID,
			Name:  r.Name,
			Zone:  zoneName,
			Type:  r.Type,
			Value: r.Content,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

func (c *Client) resolveZone(ctx context.Context, name, zoneHint string) (zoneID, fqdn string, err error) {
	root, sub := domainutil.SplitDomain(name, zoneHint)
	id, _, err := c.findZone(ctx, name)
	if err != nil {
		return "", "", err
	}
	fqdn = strings.ToLower(root)
	if sub != "" && sub != "@" {
		fqdn = strings.ToLower(sub + "." + root)
	}
	return id, fqdn, nil
}

func (c *Client) findZone(ctx context.Context, fulldomain string) (zoneID, zoneName string, err error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/zones", nil)
	if err != nil {
		return "", "", err
	}

	var zones []zone
	if err := json.Unmarshal(body, &zones); err != nil {
		return "", "", fmt.Errorf("failed to decode IONOS zones: %w", err)
	}

	candidate := strings.ToLower(strings.TrimSuffix(fulldomain, "."))
	parts := strings.Split(candidate, ".")
	for i := range len(parts) - 1 {
		root := strings.Join(parts[i+1:], ".")
		for _, z := range zones {
			if strings.EqualFold(strings.TrimSuffix(z.Name, "."), root) {
				return z.ID, root, nil
			}
		}
	}
	return "", "", fmt.Errorf("IONOS zone not found for %s", fulldomain)
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
	req.Header.Set("X-API-Key", c.prefix+"."+c.secret)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("IONOS API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read IONOS response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(respBody)}
	}
	return respBody, nil
}

// httpStatusError 表示 IONOS API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("IONOS API error: status %d, body: %s", e.status, e.body)
}
