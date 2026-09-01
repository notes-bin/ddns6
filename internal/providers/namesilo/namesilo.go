// Package namesilo 实现 NameSilo DNS API 服务。
//
// 认证方式：API Key。
// 必填参数：--api-key
//
// API 文档：https://www.namesilo.com/api-reference
package namesilo

import (
	"cmp"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

const defaultBaseURL = "https://www.namesilo.com/api"

// Client NameSilo DNS API 客户端。
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 NameSilo 客户端。
func NewClient(apiKey string, options ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
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

// namesiloReply 为 NameSilo API 通用 XML 响应。
type namesiloReply struct {
	Reply struct {
		Code    int      `xml:"code"`
		Detail  string   `xml:"detail"`
		Records []record `xml:"resource_record"`
	} `xml:"reply"`
}

// listDomainsReply 为 listDomains API 响应。
type listDomainsReply struct {
	Reply struct {
		Code    int    `xml:"code"`
		Detail  string `xml:"detail"`
		Domains struct {
			Domain []string `xml:"domain"`
		} `xml:"domains"`
	} `xml:"reply"`
}

// record 表示 NameSilo DNS 记录。
type record struct {
	ID    string `xml:"record_id"`
	Type  string `xml:"type"`
	Host  string `xml:"host"`
	Value string `xml:"value"`
	TTL   int    `xml:"ttl"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	zone, sub := domainutil.SplitDomain(record.Name, record.Zone)
	params := url.Values{
		"version": {"1"},
		"type":    {"xml"},
		"key":     {c.apiKey},
		"domain":  {zone},
		"rrtype":  {record.Type},
		"rrhost":  {sub},
		"rrvalue": {record.Value},
		"rrttl":   {strconv.Itoa(cmp.Or(record.TTL, ddns.DefaultTTL))},
	}
	slog.Debug("adding NameSilo DNS record", "module", "namesilo", "domain", zone, "host", sub, "type", record.Type)
	reply, err := c.get(ctx, "dnsAddRecord", params)
	if err != nil {
		return err
	}
	if reply.Reply.Code != 300 {
		return fmt.Errorf("NameSilo API error: code %d, detail: %s", reply.Reply.Code, reply.Reply.Detail)
	}
	slog.Info("NameSilo DNS record added", "module", "namesilo", "domain", zone, "type", record.Type, "ipv6", record.Value)
	return nil
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	zone, _ := domainutil.SplitDomain(record.Name, record.Zone)
	params := url.Values{
		"version": {"1"},
		"type":    {"xml"},
		"key":     {c.apiKey},
		"domain":  {zone},
		"rrid":    {record.ID},
		"rrvalue": {record.Value},
		"rrttl":   {strconv.Itoa(cmp.Or(record.TTL, ddns.DefaultTTL))},
	}
	reply, err := c.get(ctx, "dnsUpdateRecord", params)
	if err != nil {
		return err
	}
	if reply.Reply.Code != 300 {
		return fmt.Errorf("NameSilo API error: code %d, detail: %s", reply.Reply.Code, reply.Reply.Detail)
	}
	return nil
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	zone, _ := domainutil.SplitDomain(record.Name, record.Zone)
	params := url.Values{
		"version": {"1"},
		"type":    {"xml"},
		"key":     {c.apiKey},
		"domain":  {zone},
		"rrid":    {record.ID},
	}
	reply, err := c.get(ctx, "dnsDeleteRecord", params)
	if err != nil {
		return err
	}
	if reply.Reply.Code != 300 {
		return fmt.Errorf("NameSilo API error: code %d, detail: %s", reply.Reply.Code, reply.Reply.Detail)
	}
	return nil
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zone, _, err := c.findZone(ctx, fulldomain)
	if err != nil {
		return nil, err
	}

	params := url.Values{
		"version": {"1"},
		"type":    {"xml"},
		"key":     {c.apiKey},
		"domain":  {zone},
	}
	body, err := c.fetch(ctx, "dnsListRecords", params)
	if err != nil {
		return nil, err
	}

	var reply namesiloReply
	if err := xml.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("failed to decode NameSilo response: %w", err)
	}
	if reply.Reply.Code != 300 {
		return nil, fmt.Errorf("NameSilo API error: code %d", reply.Reply.Code)
	}

	result := make([]ddns.RecordInfo, 0, len(reply.Reply.Records))
	for _, r := range reply.Reply.Records {
		if recordType != "" && r.Type != recordType {
			continue
		}
		name := zone
		if r.Host != "" && r.Host != "@" {
			name = r.Host + "." + zone
		}
		result = append(result, ddns.RecordInfo{
			ID:    r.ID,
			Name:  name,
			Zone:  zone,
			Type:  r.Type,
			Value: r.Value,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// findZone 查找 fulldomain 对应的 NameSilo zone 与子名。
func (c *Client) findZone(ctx context.Context, fulldomain string) (zone, sub string, err error) {
	params := url.Values{
		"version": {"1"},
		"type":    {"xml"},
		"key":     {c.apiKey},
	}
	body, err := c.fetch(ctx, "listDomains", params)
	if err != nil {
		return "", "", err
	}

	var reply listDomainsReply
	if err := xml.Unmarshal(body, &reply); err != nil {
		return "", "", fmt.Errorf("failed to decode NameSilo listDomains: %w", err)
	}
	if reply.Reply.Code != 300 {
		return "", "", fmt.Errorf("NameSilo listDomains failed: code %d, detail: %s", reply.Reply.Code, reply.Reply.Detail)
	}

	parts := strings.Split(strings.TrimSuffix(fulldomain, "."), ".")
	for i := range len(parts) - 1 {
		candidate := strings.Join(parts[i+1:], ".")
		for _, d := range reply.Reply.Domains.Domain {
			if strings.EqualFold(strings.TrimSuffix(d, "."), candidate) {
				sub = strings.Join(parts[:i+1], ".")
				if sub == "" {
					sub = "@"
				}
				return candidate, sub, nil
			}
		}
	}
	return "", "", fmt.Errorf("NameSilo zone not found for %s", fulldomain)
}

// get 调用 NameSilo API 并解析 XML 响应。
func (c *Client) get(ctx context.Context, action string, params url.Values) (*namesiloReply, error) {
	body, err := c.fetch(ctx, action, params)
	if err != nil {
		return nil, err
	}
	var reply namesiloReply
	if err := xml.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("failed to decode NameSilo response: %w", err)
	}
	return &reply, nil
}

// fetch 执行 NameSilo HTTP GET 请求。
func (c *Client) fetch(ctx context.Context, action string, params url.Values) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/%s?%s", c.baseURL, action, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("NameSilo API request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read NameSilo response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(body)}
	}
	return body, nil
}

// httpStatusError 表示 NameSilo API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("NameSilo API error: status %d, body: %s", e.status, e.body)
}
