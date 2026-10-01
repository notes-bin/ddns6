// Package gcloud 实现 Google Cloud DNS API 服务。
//
// 对应 acme.sh dns_gcloud（REST API，不依赖 gcloud CLI）。
// 必填参数：--project、--access-token。
// zone 探测使用 domainutil.ZoneCandidates；路径段经 url.PathEscape 转义。
//
// API 文档：https://cloud.google.com/dns/docs/reference/v1
package gcloud

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/internal/httputil"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

// 编译期断言：Client 实现 ddns.DNSProvider。
var _ ddns.DNSProvider = (*Client)(nil)

// defaultBaseURL 为 Cloud DNS REST API v1 基址。
const defaultBaseURL = "https://dns.googleapis.com/dns/v1"

// Client Google Cloud DNS API 客户端。
type Client struct {
	project    string
	token      string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 Google Cloud DNS 客户端。
// 默认使用 httputil.NewHTTPClient（超时 + 同主机重定向限制）。
func NewClient(project, accessToken string, options ...Option) *Client {
	c := &Client{
		project:    project,
		token:      accessToken,
		baseURL:    defaultBaseURL,
		httpClient: httputil.NewHTTPClient(30 * time.Second),
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

// managedZone 表示 Google Cloud DNS 托管区域。
type managedZone struct {
	Name    string `json:"name"`
	DNSName string `json:"dnsName"`
}

// zoneList 为托管区域列表响应。
type zoneList struct {
	ManagedZones []managedZone `json:"managedZones"`
}

// rrSet 表示 Resource Record Set。
type rrSet struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     int64    `json:"ttl"`
	Rrdatas []string `json:"rrdatas"`
}

// changeRequest 为 Changes API 请求体。
type changeRequest struct {
	Additions []rrSet `json:"additions,omitempty"`
	Deletions []rrSet `json:"deletions,omitempty"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.applyChange(ctx, info, "add")
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, info ddns.RecordInfo) error {
	old := info
	old.Value = info.ValueFromID()
	if err := c.applyChange(ctx, old, "delete"); err != nil && !isNotFound(err) {
		return err
	}
	return c.applyChange(ctx, info, "add")
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.applyChange(ctx, info, "delete")
}

// GetRecords 查询 DNS 记录（列出 managed zone 下指定类型的全部 rrset）。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zone, _, zoneDNS, err := c.resolve(ctx, fulldomain, "")
	if err != nil {
		return nil, err
	}
	recordType = cmp.Or(recordType, "AAAA")
	path := fmt.Sprintf("/projects/%s/managedZones/%s/rrsets", url.PathEscape(c.project), url.PathEscape(zone))
	q := url.Values{"type": {recordType}}
	body, err := c.doRequest(ctx, http.MethodGet, path, q, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var resp struct {
		Rrsets []rrSet `json:"rrsets"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode cloud dns rrsets: %w", err)
	}
	result := make([]ddns.RecordInfo, 0)
	for _, set := range resp.Rrsets {
		for _, v := range set.Rrdatas {
			result = append(result, ddns.RecordInfo{
				ID:    set.Name + "|" + set.Type + "|" + v,
				Name:  strings.TrimSuffix(set.Name, "."),
				Zone:  strings.TrimSuffix(zoneDNS, "."),
				Type:  set.Type,
				Value: v,
				TTL:   int(set.TTL),
			})
		}
	}
	return result, nil
}

// applyChange 提交 additions 或 deletions 变更。
func (c *Client) applyChange(ctx context.Context, info ddns.RecordInfo, action string) error {
	zone, rrName, _, err := c.resolve(ctx, info.Name, info.Zone)
	if err != nil {
		return err
	}
	set := rrSet{
		Name:    rrName,
		Type:    info.Type,
		TTL:     int64(ddns.RecordTTL(info.TTL)),
		Rrdatas: []string{info.Value},
	}
	chg := changeRequest{}
	switch action {
	case "add":
		chg.Additions = []rrSet{set}
	case "delete":
		chg.Deletions = []rrSet{set}
	default:
		return fmt.Errorf("unknown change action: %s", action)
	}
	payload, err := json.Marshal(chg)
	if err != nil {
		return fmt.Errorf("failed to marshal change: %w", err)
	}
	path := fmt.Sprintf("/projects/%s/managedZones/%s/changes", url.PathEscape(c.project), url.PathEscape(zone))
	slog.Debug("applying Cloud DNS change", "module", "gcloud", "zone", zone, "action", action, "type", info.Type)
	_, err = c.doRequest(ctx, http.MethodPost, path, nil, payload)
	return err
}

// resolve 解析托管区域名与 RR 名称。
// 无 zoneHint 时按 domainutil.ZoneCandidates 从长到短匹配 managedZones。
func (c *Client) resolve(ctx context.Context, name, zoneHint string) (zoneName, rrName, zoneDNS string, err error) {
	body, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/managedZones", url.PathEscape(c.project)), nil, nil)
	if err != nil {
		return "", "", "", err
	}
	var list zoneList
	if err := json.Unmarshal(body, &list); err != nil {
		return "", "", "", fmt.Errorf("failed to decode managed zones: %w", err)
	}
	byDNS := make(map[string]managedZone, len(list.ManagedZones))
	for _, z := range list.ManagedZones {
		byDNS[strings.ToLower(strings.TrimSuffix(z.DNSName, "."))] = z
	}
	candidates := domainutil.ZoneCandidates(name)
	if zoneHint != "" {
		candidates = []string{strings.ToLower(strings.TrimSuffix(zoneHint, "."))}
	}
	for _, candidate := range candidates {
		z, ok := byDNS[candidate]
		if !ok {
			continue
		}
		_, sub := domainutil.SplitDomain(name, candidate)
		rr := candidate + "."
		if sub != "" && sub != "@" {
			rr = sub + "." + candidate + "."
		}
		return z.Name, rr, z.DNSName, nil
	}
	return "", "", "", fmt.Errorf("cloud dns managed zone not found for %s", name)
}

// doRequest 执行 Cloud DNS HTTP 请求。
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, body []byte) ([]byte, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var req *http.Request
	var err error
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
		return nil, fmt.Errorf("cloud dns request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := httputil.ReadBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(respBody)}
	}
	return respBody, nil
}

// httpStatusError 表示 Cloud DNS API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

// Error 返回含状态码与响应正文的错误描述。
func (e *httpStatusError) Error() string {
	return fmt.Sprintf("Cloud DNS API error: status %d, body: %s", e.status, e.body)
}

// isNotFound 判断错误是否为 HTTP 404。
func isNotFound(err error) bool {
	he, ok := errors.AsType[*httpStatusError](err)
	return ok && he.status == http.StatusNotFound
}
