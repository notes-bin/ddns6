package tencent

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TencentClient represents a client for Tencent Cloud DNS API
type TencentClient struct {
	SecretId   string
	SecretKey  string
	BaseURL    string
	HTTPClient *http.Client
}

type Options func(*TencentClient)

// Task implements domain.Tasker.
func (c *TencentClient) Task(domain string, subdomain string, ipv6addr string) error {
	panic("unimplemented")
}

// NewClient creates a new TencentClient
func NewClient(secretId, secretKey string, options ...Options) *TencentClient {
	client := &TencentClient{
		SecretId:   secretId,
		SecretKey:  secretKey,
		BaseURL:    "https://dnspod.tencentcloudapi.com",
		HTTPClient: &http.Client{},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL sets a custom base URL (for testing)
func WithBaseURL(baseURL string) Options {
	return func(c *TencentClient) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) Options {
	return func(c *TencentClient) {
		c.HTTPClient = httpClient
	}
}

// DNSRecord represents a Tencent Cloud DNS record
type DNSRecord struct {
	RecordId     string `json:"RecordId,omitempty"`
	Domain       string `json:"Domain,omitempty"`
	SubDomain    string `json:"SubDomain,omitempty"`
	RecordType   string `json:"RecordType,omitempty"`
	RecordLine   string `json:"RecordLine,omitempty"`
	RecordLineId string `json:"RecordLineId,omitempty"`
	Value        string `json:"Value,omitempty"`
	TTL          int    `json:"TTL,omitempty"`
}

// AddDomainRecord 添加域名解析记录
func (c *TencentClient) AddDomainRecord(fulldomain, recordType, value string, ttl int) error {
	domain, subDomain, err := c.getRootDomain(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	record := DNSRecord{
		Domain:       domain,
		SubDomain:    subDomain,
		RecordType:   recordType,
		RecordLine:   "0",
		RecordLineId: "0",
		Value:        value,
		TTL:          ttl,
	}

	_, err = c.createRecord(record)
	return err
}

// ModifyDomainRecord 修改域名解析记录
func (c *TencentClient) ModifyDomainRecord(fulldomain, recordId, recordType, newValue string, ttl int) error {
	domain, _, err := c.getRootDomain(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := map[string]any{
		"Domain":     domain,
		"RecordId":   recordId,
		"RecordType": recordType,
		"Value":      newValue,
		"TTL":        ttl,
	}

	var response struct {
		Response struct {
			RequestId string `json:"RequestId"`
		} `json:"Response"`
	}

	return c.makeRequest("ModifyRecord", payload, &response)
}

// DeleteDomainRecord 删除域名解析记录
func (c *TencentClient) DeleteDomainRecord(fulldomain, recordId string) error {
	domain, _, err := c.getRootDomain(fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}
	return c.deleteRecord(domain, recordId)
}

// GetDomainRecords 查询域名的所有解析记录
func (c *TencentClient) GetDomainRecords(fulldomain string) ([]DNSRecord, error) {
	domain, subDomain, err := c.getRootDomain(fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}
	return c.describeRecords(domain, subDomain)
}

// GetDomainRecord 查询特定解析记录
func (c *TencentClient) GetDomainRecord(fulldomain, recordId string) (*DNSRecord, error) {
	domain, _, err := c.getRootDomain(fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := map[string]any{
		"Domain":   domain,
		"RecordId": recordId,
	}

	var response struct {
		Response struct {
			RecordInfo DNSRecord `json:"RecordInfo"`
		} `json:"Response"`
	}

	err = c.makeRequest("DescribeRecord", payload, &response)
	if err != nil {
		return nil, err
	}

	return &response.Response.RecordInfo, nil
}

// FindDomainRecord 根据子域名和值查找解析记录
func (c *TencentClient) FindDomainRecord(fulldomain, recordType, value string) (*DNSRecord, error) {
	domain, subDomain, err := c.getRootDomain(fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := map[string]any{
		"Domain":      domain,
		"SubDomain":   subDomain,
		"RecordType":  recordType,
		"RecordValue": value,
	}

	var response struct {
		Response struct {
			RecordList []DNSRecord `json:"RecordList"`
		} `json:"Response"`
	}

	err = c.makeRequest("DescribeRecordFilterList", payload, &response)
	if err != nil {
		return nil, err
	}

	if len(response.Response.RecordList) > 0 {
		return &response.Response.RecordList[0], nil
	}

	return nil, nil
}

// getRootDomain finds the root domain and subdomain
func (c *TencentClient) getRootDomain(domain string) (string, string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")

		// Check if this is a valid domain
		_, err := c.describeRecords(h, "@")
		if err == nil {
			subDomain := strings.Join(parts[:i], ".")
			return h, subDomain, nil
		}
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// describeRecords lists DNS records for a domain
func (c *TencentClient) describeRecords(domain, subDomain string) ([]DNSRecord, error) {
	payload := map[string]any{
		"Domain": domain,
		"Limit":  3000,
	}

	var response struct {
		Response struct {
			RecordList []DNSRecord `json:"RecordList"`
		} `json:"Response"`
	}

	err := c.makeRequest("DescribeRecordList", payload, &response)
	if err != nil {
		return nil, err
	}

	// Filter by subdomain if specified
	if subDomain != "@" {
		var filtered []DNSRecord
		for _, record := range response.Response.RecordList {
			if record.SubDomain == subDomain {
				filtered = append(filtered, record)
			}
		}
		return filtered, nil
	}

	return response.Response.RecordList, nil
}

// createRecord creates a new DNS record
func (c *TencentClient) createRecord(record DNSRecord) (*DNSRecord, error) {
	payload := map[string]any{
		"Domain":       record.Domain,
		"SubDomain":    record.SubDomain,
		"RecordType":   record.RecordType,
		"RecordLine":   record.RecordLine,
		"RecordLineId": record.RecordLineId,
		"Value":        record.Value,
		"TTL":          record.TTL,
	}

	var response struct {
		Response struct {
			RecordId string `json:"RecordId"`
		} `json:"Response"`
	}

	err := c.makeRequest("CreateRecord", payload, &response)
	if err != nil {
		return nil, err
	}

	record.RecordId = response.Response.RecordId
	return &record, nil
}

// deleteRecord deletes a DNS record
func (c *TencentClient) deleteRecord(domain, recordId string) error {
	payload := map[string]any{
		"Domain":   domain,
		"RecordId": recordId,
	}

	var response struct {
		Response struct {
			RequestId string `json:"RequestId"`
		} `json:"Response"`
	}

	return c.makeRequest("DeleteRecord", payload, &response)
}

// findRecordId finds the ID of a specific TXT record
func (c *TencentClient) findRecordId(domain, subDomain, value string) (string, error) {
	payload := map[string]any{
		"Domain":      domain,
		"SubDomain":   subDomain,
		"RecordValue": value,
	}

	var response struct {
		Response struct {
			RecordList []struct {
				RecordId string `json:"RecordId"`
			} `json:"RecordList"`
		} `json:"Response"`
	}

	err := c.makeRequest("DescribeRecordFilterList", payload, &response)
	if err != nil {
		return "", err
	}

	if len(response.Response.RecordList) > 0 {
		return response.Response.RecordList[0].RecordId, nil
	}

	return "", nil
}

// makeRequest performs an authenticated request to the Tencent Cloud API
func (c *TencentClient) makeRequest(action string, payload any, result any) error {
	service := "dnspod"
	version := "2021-03-23"
	timestamp := time.Now().Unix()

	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Generate signature
	signature := c.generateSignatureV3(service, action, string(payloadBytes), timestamp)

	// Create request
	req, err := http.NewRequest("POST", c.BaseURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", signature)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Action", action)

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	if result != nil {
		var apiResponse struct {
			Response any `json:"Response"`
			Error    struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		}

		apiResponse.Response = result

		if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}

		if apiResponse.Error.Code != "" {
			return fmt.Errorf("API error: %s (%s)", apiResponse.Error.Message, apiResponse.Error.Code)
		}
	}

	return nil
}

// generateSignatureV3 generates a Tencent Cloud API v3 signature
func (c *TencentClient) generateSignatureV3(service, action, payload string, timestamp int64) string {
	algorithm := "TC3-HMAC-SHA256"
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	domain := service + ".tencentcloudapi.com"

	// Canonical request
	canonicalURI := "/"
	canonicalQuery := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json\nhost:%s\nx-tc-action:%s\n", domain, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"
	hashedPayload := sha256Hex(payload)
	canonicalRequest := fmt.Sprintf("POST\n%s\n%s\n%s\n%s\n%s", canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, hashedPayload)

	// String to sign
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedRequest := sha256Hex(canonicalRequest)
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s", algorithm, timestamp, credentialScope, hashedRequest)

	// Calculate signature
	secretDate := hmacSha256("TC3"+c.SecretKey, date)
	secretService := hmacSha256Hex(string(secretDate), service)
	secretSigning := hmacSha256Hex(secretService, "tc3_request")
	signature := hmacSha256Hex(secretSigning, stringToSign)

	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, c.SecretId, credentialScope, signedHeaders, signature)
}

// Helper functions for signature calculation
func sha256Hex(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func hmacSha256(key, data string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func hmacSha256Hex(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
