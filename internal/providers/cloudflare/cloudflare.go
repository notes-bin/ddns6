package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
func (c *CloudflareClient) Task(domain string, subdomain string, ipv6addr string) error {
	panic("unimplemented")
}

type Options func(*CloudflareClient)

// NewClient creates a new CloudflareClient
func NewClient(options ...Options) *CloudflareClient {
	client := &CloudflareClient{
		BaseURL:    "https://api.cloudflare.com/client/v4",
		HTTPClient: &http.Client{},
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
func (c *CloudflareClient) AddDomainRecord(fulldomain, recordType, value string, ttl int) error {
	zoneID, err := c.getZoneID(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	// 检查记录是否已存在
	records, err := c.getRecords(zoneID, fulldomain, recordType, value)
	if err != nil {
		return fmt.Errorf("failed to check existing records: %v", err)
	}

	if len(records) > 0 {
		return nil // 记录已存在
	}

	// 创建新记录
	record := DNSRecord{
		Type:    recordType,
		Name:    fulldomain,
		Content: value,
		TTL:     ttl,
	}

	_, err = c.createDNSRecord(zoneID, record)
	return err
}

// ModifyDomainRecord 修改域名解析记录
func (c *CloudflareClient) ModifyDomainRecord(fulldomain, recordID, recordType, newValue string, ttl int) error {
	zoneID, err := c.getZoneID(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	// 获取现有记录
	record, err := c.getRecordByID(zoneID, recordID)
	if err != nil {
		return fmt.Errorf("failed to get record: %v", err)
	}

	// 更新记录
	record.Content = newValue
	record.TTL = ttl

	_, err = c.updateDNSRecord(zoneID, recordID, *record)
	return err
}

// DeleteDomainRecord 删除域名解析记录
func (c *CloudflareClient) DeleteDomainRecord(fulldomain, recordID string) error {
	zoneID, err := c.getZoneID(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	return c.deleteDNSRecord(zoneID, recordID)
}

// GetDomainRecords 获取域名的所有解析记录
func (c *CloudflareClient) GetDomainRecords(fulldomain, recordType string) ([]DNSRecord, error) {
	zoneID, err := c.getZoneID(fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %v", err)
	}

	return c.getRecords(zoneID, fulldomain, recordType, "")
}

// GetDomainRecord 获取特定解析记录
func (c *CloudflareClient) GetDomainRecord(fulldomain, recordID string) (*DNSRecord, error) {
	zoneID, err := c.getZoneID(fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %v", err)
	}

	return c.getRecordByID(zoneID, recordID)
}

// getRecords 获取指定类型的记录
func (c *CloudflareClient) getRecords(zoneID, name, recordType, content string) ([]DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records?type=%s&name=%s", c.BaseURL, zoneID, recordType, name)
	if content != "" {
		url += fmt.Sprintf("&content=%s", content)
	}

	var result []DNSRecord
	err := c.makeRequest("GET", url, nil, &result)
	return result, err
}

// getRecordByID 根据ID获取记录
func (c *CloudflareClient) getRecordByID(zoneID, recordID string) (*DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.BaseURL, zoneID, recordID)
	var result DNSRecord
	err := c.makeRequest("GET", url, nil, &result)
	return &result, err
}

// updateDNSRecord 更新DNS记录
func (c *CloudflareClient) updateDNSRecord(zoneID, recordID string, record DNSRecord) (*DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.BaseURL, zoneID, recordID)

	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	var result DNSRecord
	err = c.makeRequest("PUT", url, bytes.NewBuffer(body), &result)
	return &result, err
}

// getZoneID finds the zone ID for a given domain
func (c *CloudflareClient) getZoneID(domain string) (string, error) {
	// If ZoneID is already set, use it
	if c.ZoneID != "" {
		// Verify the zone exists
		_, err := c.getZoneDetails(c.ZoneID)
		if err == nil {
			return c.ZoneID, nil
		}
	}

	// Otherwise, search for the zone
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		zone := strings.Join(parts[i:], ".")
		zoneID, err := c.findZoneID(zone)
		if err == nil {
			return zoneID, nil
		}
	}

	return "", fmt.Errorf("could not find zone ID for domain %s", domain)
}

// findZoneID searches for a zone ID by name
func (c *CloudflareClient) findZoneID(zone string) (string, error) {
	url := fmt.Sprintf("%s/zones?name=%s", c.BaseURL, zone)
	if c.AccountID != "" {
		url += fmt.Sprintf("&account.id=%s", c.AccountID)
	}

	var result []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	err := c.makeRequest("GET", url, nil, &result)
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
func (c *CloudflareClient) getZoneDetails(zoneID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/zones/%s", c.BaseURL, zoneID)
	var result map[string]interface{}
	err := c.makeRequest("GET", url, nil, &result)
	return result, err
}

// getTxtRecords retrieves TXT records matching the name and optionally content
func (c *CloudflareClient) getTxtRecords(zoneID, name, content string) ([]DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records?type=TXT&name=%s", c.BaseURL, zoneID, name)
	if content != "" {
		url += fmt.Sprintf("&content=%s", content)
	}

	var result []DNSRecord
	err := c.makeRequest("GET", url, nil, &result)
	return result, err
}

// createDNSRecord creates a new DNS record
func (c *CloudflareClient) createDNSRecord(zoneID string, record DNSRecord) (*DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records", c.BaseURL, zoneID)

	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	var result DNSRecord
	err = c.makeRequest("POST", url, bytes.NewBuffer(body), &result)
	return &result, err
}

// deleteDNSRecord deletes a DNS record
func (c *CloudflareClient) deleteDNSRecord(zoneID, recordID string) error {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.BaseURL, zoneID, recordID)
	return c.makeRequest("DELETE", url, nil, nil)
}

// makeRequest performs an HTTP request to the Cloudflare API
func (c *CloudflareClient) makeRequest(method, url string, body io.Reader, result interface{}) error {
	req, err := http.NewRequest(method, url, body)
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
