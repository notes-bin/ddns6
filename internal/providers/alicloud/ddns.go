package alicloud

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AliDNSClient represents a client for Alibaba Cloud DNS API
type AliDNSClient struct {
	AccessKeyId     string
	AccessKeySecret string
	BaseURL         string
	HTTPClient      *http.Client
}

// NewClient creates a new AliDNSClient
func NewClient(accessKeyId, accessKeySecret string, options ...func(*AliDNSClient)) *AliDNSClient {
	client := &AliDNSClient{
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		BaseURL:         "https://alidns.aliyuncs.com/",
		HTTPClient:      &http.Client{},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL sets a custom base URL (for testing)
func WithBaseURL(baseURL string) func(*AliDNSClient) {
	return func(c *AliDNSClient) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) func(*AliDNSClient) {
	return func(c *AliDNSClient) {
		c.HTTPClient = httpClient
	}
}

// DNSRecord represents an Alibaba Cloud DNS record
type DNSRecord struct {
	RecordId string `json:"RecordId"`
	Domain   string `json:"Domain"`
	RR       string `json:"RR"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
}

// AddTxtRecord adds a TXT record to Alibaba Cloud DNS
func (c *AliDNSClient) AddTxtRecord(fulldomain, txtvalue string) error {
	domain, subDomain, err := c.getRootDomain(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	params := map[string]string{
		"Action":       "AddDomainRecord",
		"DomainName":   domain,
		"RR":           subDomain,
		"Type":         "TXT",
		"Value":        txtvalue,
		"TTL":          "600",
		"RecordLine":   "default",
		"RecordLineId": "0",
	}

	_, err = c.makeRequest(params)
	return err
}

// RemoveTxtRecord removes a TXT record from Alibaba Cloud DNS
func (c *AliDNSClient) RemoveTxtRecord(fulldomain, txtvalue string) error {
	domain, subDomain, err := c.getRootDomain(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	// Find the record ID
	recordId, err := c.findRecordId(domain, subDomain, txtvalue)
	if err != nil {
		return fmt.Errorf("failed to find record: %v", err)
	}

	if recordId == "" {
		// Record doesn't exist, nothing to do
		return nil
	}

	// Delete the record
	params := map[string]string{
		"Action":   "DeleteDomainRecord",
		"RecordId": recordId,
	}

	_, err = c.makeRequest(params)
	return err
}

// getRootDomain finds the root domain and subdomain
func (c *AliDNSClient) getRootDomain(domain string) (string, string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")

		params := map[string]string{
			"Action":     "DescribeDomainRecords",
			"DomainName": h,
		}

		resp, err := c.makeRequest(params)
		if err != nil {
			continue
		}

		var result struct {
			TotalCount int `json:"TotalCount"`
		}

		if err := json.Unmarshal(resp, &result); err != nil {
			continue
		}

		if result.TotalCount > 0 {
			subDomain := strings.Join(parts[:i], ".")
			return h, subDomain, nil
		}
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// findRecordId finds the ID of a specific TXT record
func (c *AliDNSClient) findRecordId(domain, subDomain, value string) (string, error) {
	params := map[string]string{
		"Action":      "DescribeDomainRecords",
		"DomainName":  domain,
		"RRKeyWord":   subDomain,
		"TypeKeyWord": "TXT",
	}

	resp, err := c.makeRequest(params)
	if err != nil {
		return "", err
	}

	var result struct {
		DomainRecords struct {
			Record []DNSRecord `json:"Record"`
		} `json:"DomainRecords"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	for _, record := range result.DomainRecords.Record {
		if record.RR == subDomain && record.Value == value {
			return record.RecordId, nil
		}
	}

	return "", nil
}

// makeRequest performs an authenticated request to Alibaba Cloud API
func (c *AliDNSClient) makeRequest(params map[string]string) ([]byte, error) {
	// Add common parameters
	params["Format"] = "JSON"
	params["Version"] = "2015-01-09"
	params["AccessKeyId"] = c.AccessKeyId
	params["SignatureMethod"] = "HMAC-SHA1"
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = fmt.Sprintf("%d", time.Now().UnixNano())

	// Sort parameters
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var queryParts []string
	for _, k := range keys {
		queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, url.QueryEscape(params[k])))
	}
	queryString := strings.Join(queryParts, "&")

	// Calculate signature
	stringToSign := fmt.Sprintf("GET&%%2F&%s", url.QueryEscape(queryString))
	mac := hmac.New(sha1.New, []byte(c.AccessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	signature = url.QueryEscape(signature)

	// Build final URL
	fullURL := fmt.Sprintf("%s?%s&Signature=%s", c.BaseURL, queryString, signature)

	// Make request
	resp, err := c.HTTPClient.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check for API errors
	var apiError struct {
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Message != "" {
		return nil, fmt.Errorf("API error: %s", apiError.Message)
	}

	return body, nil
}

// Helper functions for URL encoding (compatible with shell version)
func urlEncodeUpperHex(s string) string {
	var buf bytes.Buffer
	for _, b := range []byte(s) {
		if shouldEncode(b) {
			buf.WriteString(fmt.Sprintf("%%%02X", b))
		} else {
			buf.WriteByte(b)
		}
	}
	return buf.String()
}

func shouldEncode(b byte) bool {
	// Keep alphanumeric and these special characters: -_.~
	if 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || '0' <= b && b <= '9' {
		return false
	}
	switch b {
	case '-', '_', '.', '~':
		return false
	}
	return true
}

// Helper functions for HMAC-SHA1 calculation
func hexDump(s string) string {
	return hex.EncodeToString([]byte(s))
}
