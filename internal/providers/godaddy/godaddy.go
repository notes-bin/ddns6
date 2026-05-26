package godaddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GoDaddyClient represents a client for GoDaddy DNS API
type GoDaddyClient struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	HTTPClient *http.Client
}

type Options func(*GoDaddyClient)

// NewClient creates a new GoDaddyClient
func NewClient(apiKey, apiSecret string, options ...Options) *GoDaddyClient {
	client := &GoDaddyClient{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    "https://api.godaddy.com/v1",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL sets a custom base URL (for testing)
func WithBaseURL(baseURL string) Options {
	return func(c *GoDaddyClient) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) Options {
	return func(c *GoDaddyClient) {
		c.HTTPClient = httpClient
	}
}

func (c *GoDaddyClient) Task(ctx context.Context, domain, subdomain, ipv6addr string) error {
	fulldomain := domain
	if subdomain != "@" {
		fulldomain = subdomain + "." + domain
	}

	records, err := c.GetDomainRecords(ctx, fulldomain, "AAAA")
	if err != nil {
		return fmt.Errorf("get domain records: %w", err)
	}

	for _, r := range records {
		if r.Data == ipv6addr {
			return nil
		}
	}
	for _, r := range records {
		return c.ModifyDomainRecord(ctx, fulldomain, "AAAA", r.Data, ipv6addr, r.TTL)
	}
	return c.AddDomainRecord(ctx, fulldomain, "AAAA", ipv6addr, 600)
}

// DNSRecord represents a GoDaddy DNS record
type DNSRecord struct {
	Data string `json:"data"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	TTL  int    `json:"ttl,omitempty"`
}

// AddDomainRecord 添加域名解析记录
func (c *GoDaddyClient) AddDomainRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	subDomain, domain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	existingRecords, err := c.getRecords(ctx, domain, subDomain, recordType)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	for _, record := range existingRecords {
		if record.Data == value {
			return nil
		}
	}

	newRecord := DNSRecord{
		Data: value,
		Type: recordType,
		Name: subDomain,
		TTL:  ttl,
	}
	newRecords := append(existingRecords, newRecord)

	return c.updateRecords(ctx, domain, subDomain, recordType, newRecords)
}

// ModifyDomainRecord 修改域名解析记录
func (c *GoDaddyClient) ModifyDomainRecord(ctx context.Context, fulldomain, recordType, oldValue, newValue string, ttl int) error {
	subDomain, domain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	existingRecords, err := c.getRecords(ctx, domain, subDomain, recordType)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	var modified bool
	for i, record := range existingRecords {
		if record.Data == oldValue {
			existingRecords[i].Data = newValue
			existingRecords[i].TTL = ttl
			modified = true
			break
		}
	}

	if !modified {
		return fmt.Errorf("record not found")
	}

	return c.updateRecords(ctx, domain, subDomain, recordType, existingRecords)
}

// DeleteDomainRecord 删除域名解析记录
func (c *GoDaddyClient) DeleteDomainRecord(ctx context.Context, fulldomain, recordType, value string) error {
	subDomain, domain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	existingRecords, err := c.getRecords(ctx, domain, subDomain, recordType)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	var newRecords []DNSRecord
	var found bool
	for _, record := range existingRecords {
		if record.Data != value {
			newRecords = append(newRecords, record)
		} else {
			found = true
		}
	}

	if !found {
		return nil
	}

	if len(newRecords) == 0 {
		return c.deleteRecords(ctx, domain, subDomain, recordType)
	}

	return c.updateRecords(ctx, domain, subDomain, recordType, newRecords)
}

// GetDomainRecords 获取域名的所有解析记录
func (c *GoDaddyClient) GetDomainRecords(ctx context.Context, fulldomain, recordType string) ([]DNSRecord, error) {
	subDomain, domain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}

	return c.getRecords(ctx, domain, subDomain, recordType)
}

// getRecords 获取指定类型的记录
func (c *GoDaddyClient) getRecords(ctx context.Context, domain, subDomain, recordType string) ([]DNSRecord, error) {
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", c.BaseURL, domain, recordType, subDomain)
	var records []DNSRecord
	err := c.makeRequest(ctx, "GET", url, nil, &records)
	return records, err
}

// updateRecords 更新记录
func (c *GoDaddyClient) updateRecords(ctx context.Context, domain, subDomain, recordType string, records []DNSRecord) error {
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", c.BaseURL, domain, recordType, subDomain)
	body, err := json.Marshal(records)
	if err != nil {
		return err
	}
	return c.makeRequest(ctx, "PUT", url, bytes.NewBuffer(body), nil)
}

// deleteRecords 删除所有记录
func (c *GoDaddyClient) deleteRecords(ctx context.Context, domain, subDomain, recordType string) error {
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", c.BaseURL, domain, recordType, subDomain)
	return c.makeRequest(ctx, "DELETE", url, nil, nil)
}

// getRootDomain finds the root domain and subdomain
func (c *GoDaddyClient) getRootDomain(ctx context.Context, domain string) (string, string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")

		_, err := c.getDomain(ctx, h)
		if err == nil {
			subDomain := strings.Join(parts[:i], ".")
			return subDomain, h, nil
		}
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// getDomain checks if a domain exists in GoDaddy
func (c *GoDaddyClient) getDomain(ctx context.Context, domain string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/domains/%s", c.BaseURL, domain)
	var result map[string]interface{}
	err := c.makeRequest(ctx, "GET", url, nil, &result)
	return result, err
}

// makeRequest performs an HTTP request to the GoDaddy API
func (c *GoDaddyClient) makeRequest(ctx context.Context, method, url string, body io.Reader, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", c.APIKey, c.APISecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return err
		}
	}

	return nil
}
