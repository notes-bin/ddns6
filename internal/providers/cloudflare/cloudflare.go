package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CloudflareClient represents a client for Cloudflare DNS API
type CloudflareClient struct {
	APIKey     string
	Email      string
	APIToken   string
	AccountID  string
	ZoneID     string
	BaseURL    string
	HTTPClient *http.Client
}

// Task implements domain.Tasker.
func (c *CloudflareClient) Task(ctx context.Context, domain, subdomain, ipv6addr string) error {
	fulldomain := domain
	if subdomain != "@" {
		fulldomain = subdomain + "." + domain
	}

	records, err := c.GetDomainRecords(ctx, fulldomain, "AAAA")
	if err != nil {
		return fmt.Errorf("get domain records: %w", err)
	}

	for _, r := range records {
		if r.Content == ipv6addr {
			return nil
		}
	}
	for _, r := range records {
		return c.ModifyDomainRecord(ctx, fulldomain, r.ID, "AAAA", ipv6addr, r.TTL)
	}
	return c.AddDomainRecord(ctx, fulldomain, "AAAA", ipv6addr, 600)
}

type Options func(*CloudflareClient)

// NewClient creates a new CloudflareClient
func NewClient(options ...Options) *CloudflareClient {
	client := &CloudflareClient{
		BaseURL:    "https://api.cloudflare.com/client/v4",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithAPIKey sets the API key and email (legacy auth)
func WithAPIKey(apiKey, email string) Options {
	return func(c *CloudflareClient) {
		c.APIKey = apiKey
		c.Email = email
	}
}

// WithAPIToken sets the API token (new auth)
func WithAPIToken(apiToken string) Options {
	return func(c *CloudflareClient) {
		c.APIToken = apiToken
	}
}

// WithAccountID sets the Account ID
func WithAccountID(accountID string) Options {
	return func(c *CloudflareClient) {
		c.AccountID = accountID
	}
}

// WithZoneID sets the Zone ID
func WithZoneID(zoneID string) Options {
	return func(c *CloudflareClient) {
		c.ZoneID = zoneID
	}
}

// WithBaseURL sets the base URL for API requests
func WithBaseURL(baseURL string) Options {
	return func(c *CloudflareClient) {
		c.BaseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) Options {
	return func(c *CloudflareClient) {
		c.HTTPClient = httpClient
	}
}

// DNSRecord represents a Cloudflare DNS record
type DNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl,omitempty"`
}

// APIResponse represents a standard Cloudflare API response
type APIResponse struct {
	Success  bool            `json:"success"`
	Errors   []ErrorDetails  `json:"errors"`
	Messages []string        `json:"messages"`
	Result   json.RawMessage `json:"result"`
}

// ErrorDetails represents error details from Cloudflare API
type ErrorDetails struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// AddDomainRecord 添加域名解析记录
func (c *CloudflareClient) AddDomainRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	zoneID, err := c.getZoneID(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	records, err := c.getRecords(ctx, zoneID, fulldomain, recordType, value)
	if err != nil {
		return fmt.Errorf("failed to check existing records: %v", err)
	}

	if len(records) > 0 {
		return nil
	}

	record := DNSRecord{
		Type:    recordType,
		Name:    fulldomain,
		Content: value,
		TTL:     ttl,
	}

	_, err = c.createDNSRecord(ctx, zoneID, record)
	return err
}

// ModifyDomainRecord 修改域名解析记录
func (c *CloudflareClient) ModifyDomainRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	zoneID, err := c.getZoneID(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	record, err := c.getRecordByID(ctx, zoneID, recordID)
	if err != nil {
		return fmt.Errorf("failed to get record: %v", err)
	}

	record.Content = newValue
	record.TTL = ttl

	_, err = c.updateDNSRecord(ctx, zoneID, recordID, *record)
	return err
}

// DeleteDomainRecord 删除域名解析记录
func (c *CloudflareClient) DeleteDomainRecord(ctx context.Context, fulldomain, recordID string) error {
	zoneID, err := c.getZoneID(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	return c.deleteDNSRecord(ctx, zoneID, recordID)
}

// GetDomainRecords 获取域名的所有解析记录
func (c *CloudflareClient) GetDomainRecords(ctx context.Context, fulldomain, recordType string) ([]DNSRecord, error) {
	zoneID, err := c.getZoneID(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %v", err)
	}

	return c.getRecords(ctx, zoneID, fulldomain, recordType, "")
}

// GetDomainRecord 获取特定解析记录
func (c *CloudflareClient) GetDomainRecord(ctx context.Context, fulldomain, recordID string) (*DNSRecord, error) {
	zoneID, err := c.getZoneID(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %v", err)
	}

	return c.getRecordByID(ctx, zoneID, recordID)
}

// resultInfo represents Cloudflare API pagination info
type resultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
	TotalCount int `json:"total_count"`
}

// getRecords 获取指定类型的记录
func (c *CloudflareClient) getRecords(ctx context.Context, zoneID, name, recordType, content string) ([]DNSRecord, error) {
	var allRecords []DNSRecord
	page := 1

	for {
		query := url.Values{}
		query.Set("type", recordType)
		query.Set("name", name)
		query.Set("per_page", "100")
		query.Set("page", strconv.Itoa(page))
		if content != "" {
			query.Set("content", content)
		}
		reqURL := fmt.Sprintf("%s/zones/%s/dns_records?%s", c.BaseURL, zoneID, query.Encode())

		records, info, err := c.listRequest(ctx, reqURL)
		if err != nil {
			return nil, err
		}
		allRecords = append(allRecords, records...)

		if info == nil || page >= info.TotalPages {
			break
		}
		page++
	}

	return allRecords, nil
}

// listRequest performs a GET request and returns records with pagination info
func (c *CloudflareClient) listRequest(ctx context.Context, reqURL string) ([]DNSRecord, *resultInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	} else {
		req.Header.Set("X-Auth-Email", c.Email)
		req.Header.Set("X-Auth-Key", c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	var apiResp struct {
		Success    bool           `json:"success"`
		Errors     []ErrorDetails `json:"errors"`
		Result     []DNSRecord    `json:"result"`
		ResultInfo *resultInfo    `json:"result_info"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, nil, err
	}

	if !apiResp.Success {
		if len(apiResp.Errors) > 0 {
			return nil, nil, fmt.Errorf("Cloudflare API error: %s", apiResp.Errors[0].Message)
		}
		return nil, nil, fmt.Errorf("Cloudflare API request was not successful")
	}

	return apiResp.Result, apiResp.ResultInfo, nil
}

// getRecordByID 根据ID获取记录
func (c *CloudflareClient) getRecordByID(ctx context.Context, zoneID, recordID string) (*DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.BaseURL, zoneID, recordID)
	var result DNSRecord
	err := c.makeRequest(ctx, "GET", url, nil, &result)
	return &result, err
}

// updateDNSRecord 更新DNS记录
func (c *CloudflareClient) updateDNSRecord(ctx context.Context, zoneID, recordID string, record DNSRecord) (*DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.BaseURL, zoneID, recordID)

	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	var result DNSRecord
	err = c.makeRequest(ctx, "PUT", url, bytes.NewBuffer(body), &result)
	return &result, err
}

// getZoneID finds the zone ID for a given domain
func (c *CloudflareClient) getZoneID(ctx context.Context, domain string) (string, error) {
	if c.ZoneID != "" {
		_, err := c.getZoneDetails(ctx, c.ZoneID)
		if err == nil {
			return c.ZoneID, nil
		}
	}

	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		zone := strings.Join(parts[i:], ".")
		zoneID, err := c.findZoneID(ctx, zone)
		if err == nil {
			return zoneID, nil
		}
	}

	return "", fmt.Errorf("could not find zone ID for domain %s", domain)
}

// findZoneID searches for a zone ID by name
func (c *CloudflareClient) findZoneID(ctx context.Context, zone string) (string, error) {
	query := url.Values{}
	query.Set("name", zone)
	if c.AccountID != "" {
		query.Set("account.id", c.AccountID)
	}
	reqURL := fmt.Sprintf("%s/zones?%s", c.BaseURL, query.Encode())

	var result []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	err := c.makeRequest(ctx, "GET", reqURL, nil, &result)
	if err != nil {
		return "", err
	}

	for _, z := range result {
		if z.Name == zone {
			return z.ID, nil
		}
	}

	return "", fmt.Errorf("zone not found")
}

// getZoneDetails gets details for a specific zone
func (c *CloudflareClient) getZoneDetails(ctx context.Context, zoneID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/zones/%s", c.BaseURL, zoneID)
	var result map[string]interface{}
	err := c.makeRequest(ctx, "GET", url, nil, &result)
	return result, err
}

// getTxtRecords retrieves TXT records matching the name and optionally content
func (c *CloudflareClient) getTxtRecords(ctx context.Context, zoneID, name, content string) ([]DNSRecord, error) {
	query := url.Values{}
	query.Set("type", "TXT")
	query.Set("name", name)
	query.Set("per_page", "100")
	if content != "" {
		query.Set("content", content)
	}
	reqURL := fmt.Sprintf("%s/zones/%s/dns_records?%s", c.BaseURL, zoneID, query.Encode())

	records, _, err := c.listRequest(ctx, reqURL)
	return records, err
}

// createDNSRecord creates a new DNS record
func (c *CloudflareClient) createDNSRecord(ctx context.Context, zoneID string, record DNSRecord) (*DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records", c.BaseURL, zoneID)

	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	var result DNSRecord
	err = c.makeRequest(ctx, "POST", url, bytes.NewBuffer(body), &result)
	return &result, err
}

// deleteDNSRecord deletes a DNS record
func (c *CloudflareClient) deleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.BaseURL, zoneID, recordID)
	return c.makeRequest(ctx, "DELETE", url, nil, nil)
}

// makeRequest performs an HTTP request to the Cloudflare API
func (c *CloudflareClient) makeRequest(ctx context.Context, method, url string, body io.Reader, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	} else {
		req.Header.Set("X-Auth-Email", c.Email)
		req.Header.Set("X-Auth-Key", c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiResp APIResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err == nil {
			if len(apiResp.Errors) > 0 {
				return fmt.Errorf("Cloudflare API error: %s (code %d)",
					apiResp.Errors[0].Message, apiResp.Errors[0].Code)
			}
		}
		return fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	if result != nil {
		var apiResp APIResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return err
		}

		if !apiResp.Success {
			if len(apiResp.Errors) > 0 {
				return fmt.Errorf("Cloudflare API error: %s", apiResp.Errors[0].Message)
			}
			return fmt.Errorf("Cloudflare API request was not successful")
		}

		if apiResp.Result != nil {
			if err := json.Unmarshal(apiResp.Result, result); err != nil {
				return err
			}
		}
	}

	return nil
}
