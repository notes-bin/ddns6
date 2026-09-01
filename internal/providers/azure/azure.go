// Package azure 实现 Azure DNS API 服务。
//
// 对应 acme.sh dns_azure，使用 Service Principal 认证。
package azure

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
	"sync"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

const (
	management = "https://management.azure.com"
	apiVersion = "2018-05-01"
	tokenScope = "https://management.azure.com/.default"
)

// Client Azure DNS API 客户端。
type Client struct {
	subscriptionID string
	tenantID       string
	clientID       string
	clientSecret   string
	loginBase      string
	managementBase string
	token          string
	tokenExpiry    time.Time
	mu             sync.Mutex
	httpClient     *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 Azure DNS 客户端。
func NewClient(subscriptionID, tenantID, clientID, clientSecret string, options ...Option) *Client {
	c := &Client{
		subscriptionID: subscriptionID,
		tenantID:       tenantID,
		clientID:       clientID,
		clientSecret:   clientSecret,
		loginBase:      "https://login.microsoftonline.com",
		managementBase: management,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

// WithManagementBase 设置 Management API 基址（测试用）。
func WithManagementBase(base string) Option {
	return func(c *Client) {
		c.managementBase = strings.TrimSuffix(base, "/")
	}
}

// WithLoginBase 设置 OAuth2 登录基址（测试用）。
func WithLoginBase(base string) Option {
	return func(c *Client) {
		c.loginBase = strings.TrimSuffix(base, "/")
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

type dnsZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type zoneList struct {
	Value []dnsZone `json:"value"`
}

type aaaaRecord struct {
	IPv6Address string `json:"ipv6Address"`
}

type recordSet struct {
	Properties struct {
		TTL         int          `json:"ttl"`
		AAAARecords []aaaaRecord `json:"aaaaRecords"`
		ARecords    []struct {
			IPv4Address string `json:"ipv4Address"`
		} `json:"aRecords"`
	} `json:"properties"`
}

// AddRecord 添加 DNS 记录。
func (c *Client) AddRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.upsert(ctx, info)
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.upsert(ctx, info)
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, info ddns.RecordInfo) error {
	path, _, _, err := c.recordPath(ctx, info.Name, info.Zone, info.Type)
	if err != nil {
		return err
	}
	_, err = c.doJSON(ctx, http.MethodDelete, path, nil)
	return err
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	if recordType == "" {
		recordType = "AAAA"
	}
	path, zone, displayName, err := c.recordPath(ctx, fulldomain, "", recordType)
	if err != nil {
		return nil, err
	}
	body, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var rs recordSet
	if err := json.Unmarshal(body, &rs); err != nil {
		return nil, fmt.Errorf("failed to decode Azure record set: %w", err)
	}
	result := make([]ddns.RecordInfo, 0, len(rs.Properties.AAAARecords))
	for _, rec := range rs.Properties.AAAARecords {
		result = append(result, ddns.RecordInfo{
			ID:    path,
			Name:  displayName,
			Zone:  zone,
			Type:  recordType,
			Value: rec.IPv6Address,
			TTL:   rs.Properties.TTL,
		})
	}
	return result, nil
}

func (c *Client) upsert(ctx context.Context, info ddns.RecordInfo) error {
	path, _, _, err := c.recordPath(ctx, info.Name, info.Zone, info.Type)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"properties": map[string]any{
			"TTL": cmp.Or(info.TTL, ddns.DefaultTTL),
		},
	}
	switch info.Type {
	case "AAAA":
		payload["properties"].(map[string]any)["AAAARecords"] = []map[string]string{{"ipv6Address": info.Value}}
	case "A":
		payload["properties"].(map[string]any)["ARecords"] = []map[string]string{{"ipv4Address": info.Value}}
	default:
		return fmt.Errorf("unsupported Azure record type: %s", info.Type)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	slog.Debug("upserting Azure DNS record", "module", "azure", "path", path, "type", info.Type)
	_, err = c.doJSON(ctx, http.MethodPut, path, body)
	return err
}

func (c *Client) recordPath(ctx context.Context, name, zoneHint, recordType string) (path, zone, displayName string, err error) {
	zoneID, zoneName, sub, err := c.findZone(ctx, name, zoneHint)
	if err != nil {
		return "", "", "", err
	}
	rr := "@"
	if sub != "" && sub != "@" {
		rr = sub
	}
	displayName = zoneName
	if rr != "@" {
		displayName = rr + "." + zoneName
	}
	path = fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnsZones/%s/%s/%s?api-version=%s",
		c.subscriptionID, extractResourceGroup(zoneID), zoneName, recordType, rr, apiVersion)
	return path, zoneName, displayName, nil
}

func extractResourceGroup(zoneID string) string {
	const marker = "/resourcegroups/"
	lower := strings.ToLower(zoneID)
	idx := strings.Index(lower, marker)
	if idx < 0 {
		return "dns"
	}
	rg, _, _ := strings.Cut(zoneID[idx+len(marker):], "/")
	return rg
}

func (c *Client) findZone(ctx context.Context, fulldomain, zoneHint string) (zoneID, zoneName, sub string, err error) {
	root, sub := domainutil.SplitDomain(fulldomain, zoneHint)
	candidate := strings.ToLower(strings.TrimSuffix(root, "."))
	path := fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Network/dnsZones?api-version=%s", c.subscriptionID, apiVersion)
	body, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", "", "", err
	}
	var list zoneList
	if err := json.Unmarshal(body, &list); err != nil {
		return "", "", "", fmt.Errorf("failed to decode Azure zones: %w", err)
	}
	for _, z := range list.Value {
		if strings.EqualFold(strings.TrimSuffix(z.Name, "."), candidate) {
			return z.ID, z.Name, sub, nil
		}
	}
	return "", "", "", fmt.Errorf("Azure DNS zone not found for %s", fulldomain)
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"scope":         {tokenScope},
	}
	endpoint := fmt.Sprintf("%s/%s/oauth2/v2.0/token", c.loginBase, c.tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Azure token request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Azure token error: status %d, body: %s", resp.StatusCode, string(body))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", err
	}
	c.token = tok.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn-60) * time.Second)
	return c.token, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	endpoint := c.managementBase + path
	var req *http.Request
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, endpoint, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Azure DNS request failed: %w", err)
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
	return fmt.Sprintf("Azure DNS API error: status %d, body: %s", e.status, e.body)
}

func isNotFound(err error) bool {
	var he *httpStatusError
	return errors.As(err, &he) && he.status == http.StatusNotFound
}
