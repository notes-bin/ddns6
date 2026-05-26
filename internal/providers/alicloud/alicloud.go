package alicloud

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
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

type Options func(*AliDNSClient)

// NewClient creates a new AliDNSClient
func NewClient(accessKeyId, accessKeySecret string, options ...Options) *AliDNSClient {
	client := &AliDNSClient{
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		BaseURL:         "https://alidns.aliyuncs.com/",
		HTTPClient:      &http.Client{Timeout: 30 * time.Second},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL sets a custom base URL (for testing)
func WithBaseURL(baseURL string) Options {
	return func(c *AliDNSClient) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) Options {
	return func(c *AliDNSClient) {
		c.HTTPClient = httpClient
	}
}

func (c *AliDNSClient) Task(ctx context.Context, domain, subdomain, ipv6addr string) error {
	fulldomain := domain
	if subdomain != "@" {
		fulldomain = subdomain + "." + domain
	}

	records, err := c.GetDomainRecords(ctx, fulldomain, "AAAA")
	if err != nil {
		return fmt.Errorf("get domain records: %w", err)
	}

	for _, r := range records {
		if r.Value == ipv6addr {
			return nil
		}
	}
	for _, r := range records {
		return c.ModifyDomainRecord(ctx, fulldomain, r.RecordId, "AAAA", ipv6addr, r.TTL)
	}
	return c.AddDomainRecord(ctx, fulldomain, "AAAA", ipv6addr, 600)
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

// AddDomainRecord 添加域名解析记录
func (c *AliDNSClient) AddDomainRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	domain, subDomain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	params := map[string]string{
		"Action":       "AddDomainRecord",
		"DomainName":   domain,
		"RR":           subDomain,
		"Type":         recordType,
		"Value":        value,
		"TTL":          fmt.Sprintf("%d", ttl),
		"RecordLine":   "default",
		"RecordLineId": "0",
	}

	_, err = c.makeRequest(ctx, params)
	return err
}

// ModifyDomainRecord 修改域名解析记录
func (c *AliDNSClient) ModifyDomainRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	_, subDomain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	params := map[string]string{
		"Action":   "UpdateDomainRecord",
		"RecordId": recordID,
		"RR":       subDomain,
		"Type":     recordType,
		"Value":    newValue,
		"TTL":      fmt.Sprintf("%d", ttl),
	}

	_, err = c.makeRequest(ctx, params)
	return err
}

// DeleteDomainRecord 删除域名解析记录
func (c *AliDNSClient) DeleteDomainRecord(ctx context.Context, fulldomain, recordID string) error {
	params := map[string]string{
		"Action":   "DeleteDomainRecord",
		"RecordId": recordID,
	}

	_, err := c.makeRequest(ctx, params)
	return err
}

// GetDomainRecords 获取域名的所有解析记录
func (c *AliDNSClient) GetDomainRecords(ctx context.Context, fulldomain, recordType string) ([]DNSRecord, error) {
	domain, subDomain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}

	params := map[string]string{
		"Action":      "DescribeDomainRecords",
		"DomainName":  domain,
		"RRKeyWord":   subDomain,
		"TypeKeyWord": recordType,
	}

	resp, err := c.makeRequest(ctx, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		DomainRecords struct {
			Record []DNSRecord `json:"Record"`
		} `json:"DomainRecords"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.DomainRecords.Record, nil
}

// GetDomainRecord 获取特定解析记录
func (c *AliDNSClient) GetDomainRecord(ctx context.Context, fulldomain, recordID string) (*DNSRecord, error) {
	params := map[string]string{
		"Action":   "DescribeDomainRecordInfo",
		"RecordId": recordID,
	}

	resp, err := c.makeRequest(ctx, params)
	if err != nil {
		return nil, err
	}

	var record DNSRecord
	if err := json.Unmarshal(resp, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// getRootDomain finds the root domain and subdomain
func (c *AliDNSClient) getRootDomain(ctx context.Context, domain string) (string, string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")

		params := map[string]string{
			"Action":     "DescribeDomainRecords",
			"DomainName": h,
		}

		resp, err := c.makeRequest(ctx, params)
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

// makeRequest performs an authenticated request to Alibaba Cloud API
func (c *AliDNSClient) makeRequest(ctx context.Context, params map[string]string) ([]byte, error) {
	// Copy params to avoid mutating caller's map
	reqParams := make(map[string]string, len(params)+8)
	for k, v := range params {
		reqParams[k] = v
	}
	reqParams["Format"] = "JSON"
	reqParams["Version"] = "2015-01-09"
	reqParams["AccessKeyId"] = c.AccessKeyId
	reqParams["SignatureMethod"] = "HMAC-SHA1"
	reqParams["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	reqParams["SignatureVersion"] = "1.0"
	reqParams["SignatureNonce"] = fmt.Sprintf("%d", time.Now().UnixNano())

	// Sort parameters
	var keys []string
	for k := range reqParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var queryParts []string
	for _, k := range keys {
		queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, url.QueryEscape(reqParams[k])))
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
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
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
