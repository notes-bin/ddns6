package godaddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GoDaddyClient represents a client for GoDaddy DNS API
type GoDaddyClient struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new GoDaddyClient
func NewClient(apiKey, apiSecret string, options ...func(*GoDaddyClient)) *GoDaddyClient {
	client := &GoDaddyClient{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    "https://api.godaddy.com/v1",
		HTTPClient: &http.Client{},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL sets a custom base URL (for testing)
func WithBaseURL(baseURL string) func(*GoDaddyClient) {
	return func(c *GoDaddyClient) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) func(*GoDaddyClient) {
	return func(c *GoDaddyClient) {
		c.HTTPClient = httpClient
	}
}

// DNSRecord represents a GoDaddy DNS record
type DNSRecord struct {
	Data string `json:"data"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	TTL  int    `json:"ttl,omitempty"`
}

// AddTxtRecord adds a TXT record to GoDaddy DNS
func (c *GoDaddyClient) AddTxtRecord(fulldomain, txtvalue string) error {
	subDomain, domain, err := c.getRootDomain(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	// Get existing records
	existingRecords, err := c.getTxtRecords(domain, subDomain)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	// Check if record already exists
	for _, record := range existingRecords {
		if record.Data == txtvalue {
			// Record already exists
			return nil
		}
	}

	// Prepare new records (keep existing and add new one)
	newRecords := append(existingRecords, DNSRecord{
		Data: txtvalue,
		Type: "TXT",
		Name: subDomain,
	})

	// Update records
	err = c.updateTxtRecords(domain, subDomain, newRecords)
	if err != nil {
		return fmt.Errorf("failed to update records: %v", err)
	}

	// Verify the record was added
	updatedRecords, err := c.getTxtRecords(domain, subDomain)
	if err != nil {
		return fmt.Errorf("failed to verify record addition: %v", err)
	}

	found := false
	for _, record := range updatedRecords {
		if record.Data == txtvalue {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("failed to add TXT record, value not found after addition")
	}

	return nil
}

// RemoveTxtRecord removes a TXT record from GoDaddy DNS
func (c *GoDaddyClient) RemoveTxtRecord(fulldomain, txtvalue string) error {
	subDomain, domain, err := c.getRootDomain(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	// Get existing records
	existingRecords, err := c.getTxtRecords(domain, subDomain)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	// Filter out the record to remove
	var newRecords []DNSRecord
	found := false
	for _, record := range existingRecords {
		if record.Data != txtvalue {
			newRecords = append(newRecords, record)
		} else {
			found = true
		}
	}

	if !found {
		// Record doesn't exist, nothing to do
		return nil
	}

	if len(newRecords) == 0 {
		// No records left, delete the entry completely
		err = c.deleteTxtRecords(domain, subDomain)
		if err != nil {
			return fmt.Errorf("failed to delete empty TXT record: %v", err)
		}
	} else {
		// Update with remaining records
		err = c.updateTxtRecords(domain, subDomain, newRecords)
		if err != nil {
			return fmt.Errorf("failed to update records: %v", err)
		}
	}

	return nil
}

// getRootDomain finds the root domain and subdomain
func (c *GoDaddyClient) getRootDomain(domain string) (string, string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")

		// Check if this is a valid domain
		_, err := c.getDomain(h)
		if err == nil {
			subDomain := strings.Join(parts[:i], ".")
			return subDomain, h, nil
		}
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// getDomain checks if a domain exists in GoDaddy
func (c *GoDaddyClient) getDomain(domain string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/domains/%s", c.BaseURL, domain)
	var result map[string]interface{}
	err := c.makeRequest("GET", url, nil, &result)
	return result, err
}

// getTxtRecords retrieves TXT records for a subdomain
func (c *GoDaddyClient) getTxtRecords(domain, subDomain string) ([]DNSRecord, error) {
	url := fmt.Sprintf("%s/domains/%s/records/TXT/%s", c.BaseURL, domain, subDomain)
	var records []DNSRecord
	err := c.makeRequest("GET", url, nil, &records)
	return records, err
}

// updateTxtRecords updates TXT records for a subdomain
func (c *GoDaddyClient) updateTxtRecords(domain, subDomain string, records []DNSRecord) error {
	url := fmt.Sprintf("%s/domains/%s/records/TXT/%s", c.BaseURL, domain, subDomain)

	body, err := json.Marshal(records)
	if err != nil {
		return err
	}

	return c.makeRequest("PUT", url, bytes.NewBuffer(body), nil)
}

// deleteTxtRecords deletes all TXT records for a subdomain
func (c *GoDaddyClient) deleteTxtRecords(domain, subDomain string) error {
	url := fmt.Sprintf("%s/domains/%s/records/TXT/%s", c.BaseURL, domain, subDomain)
	return c.makeRequest("DELETE", url, nil, nil)
}

// makeRequest performs an HTTP request to the GoDaddy API
func (c *GoDaddyClient) makeRequest(method, url string, body io.Reader, result interface{}) error {
	req, err := http.NewRequest(method, url, body)
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
