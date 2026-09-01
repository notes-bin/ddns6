// Package gcloud 实现 Google Cloud DNS API 服务。
//
// 对应 acme.sh dns_gcloud（REST API，不依赖 gcloud CLI）。
// 必填参数：--project、--access-token
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
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

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
func NewClient(project, accessToken string, options ...Option) *Client {
	c := &Client{
		project:    project,
		token:      accessToken,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
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

type managedZone struct {
	Name    string `json:"name"`
	DNSName string `json:"dnsName"`
}

type zoneList struct {
	ManagedZones []managedZone `json:"managedZones"`
}

type rrSet struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     int64    `json:"ttl"`
	Rrdatas []string `json:"rrdatas"`
}

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
	if err := c.applyChange(ctx, info, "delete"); err != nil && !isNotFound(err) {
		return err
	}
	return c.applyChange(ctx, info, "add")
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.applyChange(ctx, info, "delete")
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zone, rrName, zoneDNS, err := c.resolve(ctx, fulldomain, "")
	if err != nil {
		return nil, err
	}
	if recordType == "" {
		recordType = "AAAA"
	}
	path := fmt.Sprintf("/projects/%s/managedZones/%s/rrsets", c.project, zone)
	q := url.Values{"name": {rrName}, "type": {recordType}}
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
		return nil, fmt.Errorf("failed to decode Cloud DNS rrsets: %w", err)
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

func (c *Client) applyChange(ctx context.Context, info ddns.RecordInfo, action string) error {
	zone, rrName, _, err := c.resolve(ctx, info.Name, info.Zone)
	if err != nil {
		return err
	}
	set := rrSet{
		Name:    rrName,
		Type:    info.Type,
		TTL:     int64(cmp.Or(info.TTL, ddns.DefaultTTL)),
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
	path := fmt.Sprintf("/projects/%s/managedZones/%s/changes", c.project, zone)
	slog.Debug("applying Cloud DNS change", "module", "gcloud", "zone", zone, "action", action, "type", info.Type)
	_, err = c.doRequest(ctx, http.MethodPost, path, nil, payload)
	return err
}

func (c *Client) resolve(ctx context.Context, name, zoneHint string) (zoneName, rrName, zoneDNS string, err error) {
	root, sub := domainutil.SplitDomain(name, zoneHint)
	candidate := strings.ToLower(strings.TrimSuffix(root, ".")) + "."
	body, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/managedZones", c.project), nil, nil)
	if err != nil {
		return "", "", "", err
	}
	var list zoneList
	if err := json.Unmarshal(body, &list); err != nil {
		return "", "", "", fmt.Errorf("failed to decode managed zones: %w", err)
	}
	for _, z := range list.ManagedZones {
		if strings.EqualFold(z.DNSName, candidate) {
			rr := candidate
			if sub != "" && sub != "@" {
				rr = sub + "." + candidate
			}
			return z.Name, rr, z.DNSName, nil
		}
	}
	return "", "", "", fmt.Errorf("Cloud DNS managed zone not found for %s", name)
}

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
		return nil, fmt.Errorf("Cloud DNS request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(respBody)}
	}
	return respBody, nil
}

type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("Cloud DNS API error: status %d, body: %s", e.status, e.body)
}

func isNotFound(err error) bool {
	var he *httpStatusError
	return errors.As(err, &he) && he.status == http.StatusNotFound
}
