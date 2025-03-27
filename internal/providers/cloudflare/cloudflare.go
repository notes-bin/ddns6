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

// NewClient creates a new CloudflareClient
func NewClient(options ...func(*CloudflareClient)) *CloudflareClient {
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
func WithAPIKey(apiKey, email string) func(*CloudflareClient) {
	return func(c *CloudflareClient) {
		c.APIKey = apiKey
		c.Email = email
	}
}

// WithAPIToken sets the API token (new auth)
func WithAPIToken(apiToken string) func(*CloudflareClient) {
	return func(c *CloudflareClient) {
		c.APIToken = apiToken
	}
}

// WithAccountID sets the Account ID
func WithAccountID(accountID string) func(*CloudflareClient) {
	return func(c *CloudflareClient) {
		c.AccountID = accountID
	}
}

// WithZoneID sets the Zone ID
func WithZoneID(zoneID string) func(*CloudflareClient) {
	return func(c *CloudflareClient) {
		c.ZoneID = zoneID
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

// AddTxtRecord adds a TXT record to Cloudflare DNS
func (c *CloudflareClient) AddTxtRecord(fulldomain, txtvalue string) error {
	zoneID, err := c.getZoneID(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	// Check if record already exists
	records, err := c.getTxtRecords(zoneID, fulldomain, txtvalue)
	if err != nil {
		return fmt.Errorf("failed to check existing records: %v", err)
	}

	if len(records) > 0 {
		// Record already exists
		return nil
	}

	// Create new record
	record := DNSRecord{
		Type:    "TXT",
		Name:    fulldomain,
		Content: txtvalue,
		TTL:     120,
	}

	_, err = c.createDNSRecord(zoneID, record)
	if err != nil {
		return fmt.Errorf("failed to create DNS record: %v", err)
	}

	return nil
}

// RemoveTxtRecord removes a TXT record from Cloudflare DNS
func (c *CloudflareClient) RemoveTxtRecord(fulldomain, txtvalue string) error {
	zoneID, err := c.getZoneID(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	records, err := c.getTxtRecords(zoneID, fulldomain, txtvalue)
	if err != nil {
		return fmt.Errorf("failed to get records for deletion: %v", err)
	}

	if len(records) == 0 {
		// No records to delete
		return nil
	}

	for _, record := range records {
		err := c.deleteDNSRecord(zoneID, record.ID)
		if err != nil {
			return fmt.Errorf("failed to delete record %s: %v", record.ID, err)
		}
	}

	return nil
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
