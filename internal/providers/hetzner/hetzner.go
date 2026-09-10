// Package hetzner 实现 Hetzner Cloud DNS API 服务。
//
// 认证方式：API Token（需具有 DNS 权限）。
// 必填参数：--token
//
// API 文档：https://dns.hetzner.com/api-docs
package hetzner

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
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

const defaultBaseURL = "https://api.hetzner.cloud/v1"

// Client Hetzner Cloud DNS API 客户端。
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 Hetzner DNS 客户端。
func NewClient(token string, options ...Option) *Client {
	c := &Client{
		token:      token,
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

// zone 表示 Hetzner DNS Zone。
type zone struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// zoneResponse 为单 Zone API 响应。
type zoneResponse struct {
	Zone zone `json:"zone"`
}

// zonesResponse 为 Zone 列表 API 响应。
type zonesResponse struct {
	Zones []zone `json:"zones"`
}

// rrRecord 表示 RRset 中的单条记录值。
type rrRecord struct {
	Value string `json:"value"`
}

// rrset 表示 Hetzner RRset。
type rrset struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Type    string     `json:"type"`
	TTL     int        `json:"ttl"`
	Records []rrRecord `json:"records"`
}

// rrsetResponse 为 RRset API 响应。
type rrsetResponse struct {
	RRSet rrset `json:"rrset"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, rrName, err := c.resolveRRName(ctx, record.Name, record.Zone)
	if err != nil {
		return err
	}

	ttl := cmp.Or(record.TTL, ddns.DefaultTTL)
	payload, err := json.Marshal(map[string]any{
		"ttl": ttl,
		"records": []map[string]string{
			{"value": record.Value},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	path := fmt.Sprintf("/zones/%d/rrsets/%s/%s/actions/add_records", zoneID, url.PathEscape(rrName), record.Type)
	slog.Debug("adding Hetzner DNS record", "module", "hetzner", "zone_id", zoneID, "name", rrName, "type", record.Type)
	_, err = c.doRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	slog.Info("Hetzner DNS record added", "module", "hetzner", "name", rrName, "type", record.Type, "ipv6", record.Value)
	return nil
}

// ModifyRecord 修改 DNS 记录（使用 set_records 覆盖 rrset）。
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, rrName, err := c.resolveRRName(ctx, record.Name, record.Zone)
	if err != nil {
		return err
	}

	ttl := cmp.Or(record.TTL, ddns.DefaultTTL)
	payload, err := json.Marshal(map[string]any{
		"ttl": ttl,
		"records": []map[string]string{
			{"value": record.Value},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	path := fmt.Sprintf("/zones/%d/rrsets/%s/%s/actions/set_records", zoneID, url.PathEscape(rrName), record.Type)
	_, err = c.doRequest(ctx, http.MethodPost, path, payload)
	return err
}

// DeleteRecord 删除 DNS 记录（移除指定值，若 rrset 为空则删除 rrset）。
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	zoneID, rrName, err := c.resolveRRName(ctx, record.Name, record.Zone)
	if err != nil {
		return err
	}

	oldValue := recordValueFromID(record)

	payload, err := json.Marshal(map[string]any{
		"records": []map[string]string{
			{"value": oldValue},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	path := fmt.Sprintf("/zones/%d/rrsets/%s/%s/actions/remove_records", zoneID, url.PathEscape(rrName), record.Type)
	_, err = c.doRequest(ctx, http.MethodPost, path, payload)
	return err
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zoneID, zoneName, rrName, err := c.resolveRRNameWithZone(ctx, fulldomain)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/zones/%d/rrsets/%s/%s", zoneID, url.PathEscape(rrName), recordType)
	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	var resp rrsetResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode Hetzner rrset: %w", err)
	}

	result := make([]ddns.RecordInfo, 0, len(resp.RRSet.Records))
	for _, rec := range resp.RRSet.Records {
		name := zoneName
		if rrName != "@" && rrName != "" {
			name = rrName + "." + zoneName
		}
		result = append(result, ddns.RecordInfo{
			ID:    fmt.Sprintf("%s|%s", resp.RRSet.ID, rec.Value),
			Name:  name,
			Zone:  zoneName,
			Type:  resp.RRSet.Type,
			Value: rec.Value,
			TTL:   resp.RRSet.TTL,
		})
	}
	return result, nil
}

// resolveRRName 解析 zone ID 与 RR 名称。
func (c *Client) resolveRRName(ctx context.Context, name, zoneHint string) (zoneID int64, rrName string, err error) {
	id, _, rr, err := c.resolveRRNameWithZoneFromRecord(ctx, name, zoneHint)
	return id, rr, err
}

// resolveRRNameWithZone 解析 zone ID、zone 名与 RR 名称。
func (c *Client) resolveRRNameWithZone(ctx context.Context, fulldomain string) (zoneID int64, zoneName, rrName string, err error) {
	return c.resolveRRNameWithZoneFromRecord(ctx, fulldomain, "")
}

// resolveRRNameWithZoneFromRecord 从 RecordInfo 解析 zone 与 RR 名称。
func (c *Client) resolveRRNameWithZoneFromRecord(ctx context.Context, name, zoneHint string) (zoneID int64, zoneName, rrName string, err error) {
	_, sub := domainutil.SplitDomain(name, zoneHint)
	id, zone, err := c.findZone(ctx, name)
	if err != nil {
		return 0, "", "", err
	}
	rr := sub
	if rr == "" || rr == "@" {
		rr = "@"
	}
	return id, zone, rr, nil
}

// recordValueFromID 从 RecordInfo.ID 提取旧记录值。
func recordValueFromID(record ddns.RecordInfo) string {
	if _, value, ok := strings.Cut(record.ID, "|"); ok {
		return value
	}
	return record.Value
}

// httpStatusError 表示 Hetzner API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("Hetzner API error: status %d, body: %s", e.status, e.body)
}

// isNotFound 判断错误是否为 HTTP 404。
func isNotFound(err error) bool {
	he, ok := errors.AsType[*httpStatusError](err)
	return ok && he.status == http.StatusNotFound
}

// findZone 查找 fulldomain 对应的 Hetzner zone。
func (c *Client) findZone(ctx context.Context, fulldomain string) (int64, string, error) {
	candidate := strings.ToLower(strings.TrimSuffix(fulldomain, "."))
	parts := strings.Split(candidate, ".")
	for i := range len(parts) - 1 {
		root := strings.Join(parts[i+1:], ".")
		body, err := c.doRequest(ctx, http.MethodGet, "/zones/"+url.PathEscape(root), nil)
		if err == nil {
			var resp zoneResponse
			if err := json.Unmarshal(body, &resp); err == nil && resp.Zone.ID != 0 {
				return resp.Zone.ID, strings.TrimSuffix(resp.Zone.Name, "."), nil
			}
		}

		listBody, err := c.doRequest(ctx, http.MethodGet, "/zones?name="+url.QueryEscape(root), nil)
		if err != nil {
			continue
		}
		var list zonesResponse
		if err := json.Unmarshal(listBody, &list); err != nil {
			continue
		}
		for _, z := range list.Zones {
			if strings.EqualFold(strings.TrimSuffix(z.Name, "."), root) {
				return z.ID, root, nil
			}
		}
	}
	return 0, "", fmt.Errorf("Hetzner zone not found for %s", fulldomain)
}

// doRequest 执行 Hetzner DNS HTTP 请求。
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
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Hetzner API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Hetzner response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(respBody)}
	}
	return respBody, nil
}
