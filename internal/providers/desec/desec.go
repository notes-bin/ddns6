// Package desec 实现 deSEC.io DNS API 服务。
//
// 认证方式：API Token（从 deSEC 控制台创建）。
// 必填参数：--token
//
// API 文档：https://desec.readthedocs.io/
package desec

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
	"slices"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

const defaultBaseURL = "https://desec.io/api/v1"

// Client deSEC DNS API 客户端。
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 deSEC 客户端。
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

// domainInfo 表示 deSEC 域名信息。
type domainInfo struct {
	Name string `json:"name"`
}

// rrset 表示 deSEC RRset。
type rrset struct {
	Subname string   `json:"subname"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Records []string `json:"records"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	zone, sub := splitRecord(record)
	existing, err := c.getRRSet(ctx, zone, sub, record.Type)
	if err != nil {
		return err
	}

	records := existing
	if slices.Contains(records, record.Value) {
		slog.Info("deSEC record already exists", "module", "desec", "zone", zone, "subname", sub, "type", record.Type)
		return nil
	}
	records = append(records, record.Value)

	ttl := cmp.Or(record.TTL, 3600)

	return c.putRRSets(ctx, zone, []rrset{{
		Subname: sub,
		Type:    record.Type,
		TTL:     ttl,
		Records: records,
	}})
}

// ModifyRecord 修改 DNS 记录（通过 PUT 覆盖 rrset 中的目标值）。
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	zone, sub := splitRecord(record)
	records, err := c.getRRSet(ctx, zone, sub, record.Type)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("deSEC rrset not found for %s %s", sub, record.Type)
	}

	oldValue := recordValueFromID(record)

	newRecords := make([]string, 0, len(records))
	replaced := false
	for _, v := range records {
		if !replaced && v == oldValue {
			newRecords = append(newRecords, record.Value)
			replaced = true
			continue
		}
		if v == record.Value {
			continue
		}
		newRecords = append(newRecords, v)
	}
	if !replaced {
		// 找不到旧值时，用单条记录覆盖（与 DDNS 单 AAAA 场景一致）
		newRecords = []string{record.Value}
	}

	ttl := cmp.Or(record.TTL, 3600)
	return c.putRRSets(ctx, zone, []rrset{{
		Subname: sub,
		Type:    record.Type,
		TTL:     ttl,
		Records: newRecords,
	}})
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	zone, sub := splitRecord(record)
	records, err := c.getRRSet(ctx, zone, sub, record.Type)
	if err != nil {
		return err
	}

	oldValue := recordValueFromID(record)

	newRecords := make([]string, 0, len(records))
	for _, v := range records {
		if v != oldValue && v != record.Value {
			newRecords = append(newRecords, v)
		}
	}

	ttl := cmp.Or(record.TTL, 3600)
	return c.putRRSets(ctx, zone, []rrset{{
		Subname: sub,
		Type:    record.Type,
		TTL:     ttl,
		Records: newRecords,
	}})
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zone, sub, err := c.findZone(ctx, fulldomain)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/domains/%s/rrsets/%s/%s/", zone, url.PathEscape(sub), recordType), nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	var rs rrset
	if err := json.Unmarshal(body, &rs); err != nil {
		return nil, fmt.Errorf("failed to decode deSEC rrset: %w", err)
	}

	result := make([]ddns.RecordInfo, 0, len(rs.Records))
	for _, v := range rs.Records {
		name := zone
		if sub != "" && sub != "@" {
			name = sub + "." + zone
		}
		result = append(result, ddns.RecordInfo{
			ID:    fmt.Sprintf("%s|%s", sub, v),
			Name:  name,
			Zone:  zone,
			Type:  rs.Type,
			Value: v,
			TTL:   rs.TTL,
		})
	}
	return result, nil
}

// getRRSet 获取指定 subname 与类型的 RRset 记录值。
func (c *Client) getRRSet(ctx context.Context, zone, sub, recordType string) ([]string, error) {
	body, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/domains/%s/rrsets/%s/%s/", zone, url.PathEscape(sub), recordType), nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var rs rrset
	if err := json.Unmarshal(body, &rs); err != nil {
		return nil, fmt.Errorf("failed to decode deSEC rrset: %w", err)
	}
	return rs.Records, nil
}

// putRRSets 批量 PUT 更新 RRset。
func (c *Client) putRRSets(ctx context.Context, zone string, sets []rrset) error {
	payload, err := json.Marshal(sets)
	if err != nil {
		return fmt.Errorf("failed to marshal rrsets: %w", err)
	}
	slog.Debug("updating deSEC rrsets", "module", "desec", "zone", zone, "count", len(sets))
	_, err = c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/domains/%s/rrsets/", zone), payload)
	return err
}

// findZone 查找 fulldomain 对应的 deSEC zone 与子名。
func (c *Client) findZone(ctx context.Context, fulldomain string) (zone, sub string, err error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/domains/", nil)
	if err != nil {
		return "", "", err
	}
	var domains []domainInfo
	if err := json.Unmarshal(body, &domains); err != nil {
		return "", "", fmt.Errorf("failed to decode deSEC domains: %w", err)
	}

	parts := strings.Split(strings.ToLower(strings.TrimSuffix(fulldomain, ".")), ".")
	for i := range len(parts) - 1 {
		candidate := strings.Join(parts[i+1:], ".")
		for _, d := range domains {
			if strings.EqualFold(d.Name, candidate) {
				sub = strings.Join(parts[:i+1], ".")
				if sub == "" {
					sub = "@"
				}
				return candidate, sub, nil
			}
		}
	}
	return "", "", fmt.Errorf("deSEC zone not found for %s", fulldomain)
}

// recordValueFromID 从 RecordInfo.ID 提取旧记录值。
func recordValueFromID(record ddns.RecordInfo) string {
	if _, value, ok := strings.Cut(record.ID, "|"); ok {
		return value
	}
	return record.Value
}

// splitRecord 将 RecordInfo 拆分为 zone 与 deSEC subname。
func splitRecord(record ddns.RecordInfo) (zone, sub string) {
	zone, sub = domainutil.SplitDomain(record.Name, record.Zone)
	sub = strings.ToLower(sub)
	if sub == "@" {
		sub = ""
	}
	return zone, sub
}

// httpStatusError 表示 deSEC API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("deSEC API error: status %d, body: %s", e.status, e.body)
}

// isNotFound 判断错误是否为 HTTP 404。
func isNotFound(err error) bool {
	var he *httpStatusError
	return errors.As(err, &he) && he.status == http.StatusNotFound
}

// doRequest 执行 deSEC HTTP 请求。
func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deSEC API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read deSEC response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(respBody)}
	}
	return respBody, nil
}
