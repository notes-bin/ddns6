// Package aws 实现 Amazon Route 53 DNS API 服务。
//
// 对应 acme.sh dns_aws。
// 必填参数：--access-key-id、--secret-access-key
//
// API 文档：https://docs.aws.amazon.com/Route53/latest/APIReference/
package aws

import (
	"bytes"
	"cmp"
	"context"
	"encoding/xml"
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

const defaultHost = "route53.amazonaws.com"

// Client Route 53 API 客户端。
type Client struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	host            string
	scheme          string
	httpClient      *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 Route 53 客户端。
func NewClient(accessKeyID, secretAccessKey string, options ...Option) *Client {
	c := &Client{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		host:            defaultHost,
		scheme:          "https",
		httpClient:      &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

// WithSessionToken 设置临时会话 Token（STS/IAM Role）。
func WithSessionToken(token string) Option {
	return func(c *Client) {
		c.sessionToken = token
	}
}

// WithHost 设置 API 主机（测试用）。
func WithHost(host string) Option {
	return func(c *Client) {
		c.host = host
	}
}

// WithScheme 设置 URL scheme（测试用，默认 https）。
func WithScheme(scheme string) Option {
	return func(c *Client) {
		c.scheme = scheme
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

type hostedZone struct {
	ID   string `xml:"Id"`
	Name string `xml:"Name"`
}

type listHostedZonesResponse struct {
	Zones []hostedZone `xml:"HostedZones>HostedZone"`
}

type resourceRecord struct {
	Value string `xml:"Value"`
}

type resourceRecordSet struct {
	Name            string           `xml:"Name"`
	Type            string           `xml:"Type"`
	TTL             int64            `xml:"TTL"`
	ResourceRecords []resourceRecord `xml:"ResourceRecords>ResourceRecord"`
}

type listRRSetsResponse struct {
	Sets []resourceRecordSet `xml:"ResourceRecordSets>ResourceRecordSet"`
}

// AddRecord 添加 DNS 记录（UPSERT）。
func (c *Client) AddRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.change(ctx, info, "UPSERT")
}

// ModifyRecord 修改 DNS 记录（UPSERT）。
func (c *Client) ModifyRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.change(ctx, info, "UPSERT")
}

// DeleteRecord 删除 DNS 记录（DELETE）。
func (c *Client) DeleteRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.change(ctx, info, "DELETE")
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	zoneID, zoneName, rrName, err := c.resolveRecord(ctx, fulldomain, "")
	if err != nil {
		return nil, err
	}
	q := url.Values{
		"name": {rrName},
		"type": {recordType},
	}
	body, err := c.doRequest(ctx, http.MethodGet, "/2013-04-01"+zoneID+"/rrset", q, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var resp listRRSetsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode Route53 rrsets: %w", err)
	}
	result := make([]ddns.RecordInfo, 0, len(resp.Sets))
	for _, set := range resp.Sets {
		if recordType != "" && set.Type != recordType {
			continue
		}
		for _, rec := range set.ResourceRecords {
			displayName := strings.TrimSuffix(set.Name, ".")
			result = append(result, ddns.RecordInfo{
				ID:    set.Name + "|" + set.Type + "|" + rec.Value,
				Name:  displayName,
				Zone:  strings.TrimSuffix(zoneName, "."),
				Type:  set.Type,
				Value: rec.Value,
				TTL:   int(set.TTL),
			})
		}
	}
	return result, nil
}

func (c *Client) change(ctx context.Context, info ddns.RecordInfo, action string) error {
	zoneID, _, rrName, err := c.resolveRecord(ctx, info.Name, info.Zone)
	if err != nil {
		return err
	}
	ttl := cmp.Or(info.TTL, ddns.DefaultTTL)
	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ChangeResourceRecordSetsRequest xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <ChangeBatch><Changes><Change><Action>%s</Action>
    <ResourceRecordSet>
      <Name>%s</Name><Type>%s</Type><TTL>%d</TTL>
      <ResourceRecords><ResourceRecord><Value>%s</Value></ResourceRecord></ResourceRecords>
    </ResourceRecordSet>
  </Change></Changes></ChangeBatch>
</ChangeResourceRecordSetsRequest>`, action, rrName, info.Type, ttl, info.Value)

	_, err = c.doRequest(ctx, http.MethodPost, "/2013-04-01"+zoneID+"/rrset/", nil, []byte(xmlBody))
	return err
}

func (c *Client) resolveRecord(ctx context.Context, fulldomain, zoneHint string) (zoneID, zoneName, rrName string, err error) {
	root, sub := domainutil.SplitDomain(fulldomain, zoneHint)
	candidate := strings.ToLower(strings.TrimSuffix(root, "."))
	parts := strings.Split(candidate, ".")
	for i := range len(parts) - 1 {
		zone := strings.Join(parts[i:], ".")
		id, name, err := c.findHostedZone(ctx, zone)
		if err != nil {
			return "", "", "", err
		}
		if id == "" {
			continue
		}
		rr := candidate + "."
		if sub != "" && sub != "@" {
			rr = sub + "." + zone + "."
		}
		return id, name, rr, nil
	}
	return "", "", "", fmt.Errorf("Route53 hosted zone not found for %s", fulldomain)
}

func (c *Client) findHostedZone(ctx context.Context, zone string) (id, name string, err error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/2013-04-01/hostedzone", nil, nil)
	if err != nil {
		return "", "", err
	}
	var resp listHostedZonesResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return "", "", fmt.Errorf("failed to decode hosted zones: %w", err)
	}
	want := zone + "."
	for _, z := range resp.Zones {
		if strings.EqualFold(z.Name, want) {
			return z.ID, z.Name, nil
		}
	}
	return "", "", nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, body []byte) ([]byte, error) {
	endpoint := c.scheme + "://" + c.host + path
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
	if err := signRequest(req, c.accessKeyID, c.secretAccessKey, c.sessionToken, body); err != nil {
		return nil, err
	}
	slog.Debug("Route53 API request", "module", "aws", "method", method, "path", path)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Route53 API request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Route53 response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(respBody)}
	}
	return respBody, nil
}

// httpStatusError 表示 Route 53 API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("Route53 API error: status %d, body: %s", e.status, e.body)
}

func isNotFound(err error) bool {
	var he *httpStatusError
	return errors.As(err, &he) && he.status == http.StatusNotFound
}
