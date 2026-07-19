package alicloud

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/providers"
)

var log = slog.With("module", "alicloud")

// AliDNSClient 阿里云 DNS API 客户端
type AliDNSClient struct {
	AccessKeyId     string
	AccessKeySecret string
	BaseURL         string
	HTTPClient      *http.Client
}

type Options func(*AliDNSClient)

// NewClient 创建 AliDNSClient
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

// WithBaseURL 设置自定义 API 地址（测试用）
func WithBaseURL(baseURL string) Options {
	return func(c *AliDNSClient) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(httpClient *http.Client) Options {
	return func(c *AliDNSClient) {
		c.HTTPClient = httpClient
	}
}

// DNSRecord  an Alibaba Cloud DNS record
type DNSRecord struct {
	RecordId string `json:"RecordId"`
	Domain   string `json:"DomainName"`
	RR       string `json:"RR"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
}

// AddRecord 添加域名解析记录
func (c *AliDNSClient) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
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
	}

	_, err = c.makeRequest(ctx, params)
	return err
}

// ModifyRecord 修改域名解析记录
func (c *AliDNSClient) ModifyRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
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

// DeleteRecord 删除域名解析记录
func (c *AliDNSClient) DeleteRecord(ctx context.Context, fulldomain, recordID string) error {
	params := map[string]string{
		"Action":   "DeleteDomainRecord",
		"RecordId": recordID,
	}

	_, err := c.makeRequest(ctx, params)
	return err
}

// GetRecords 查询域名的解析记录，返回通用 RecordInfo 列表
func (c *AliDNSClient) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
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

	records := make([]providers.RecordInfo, 0, len(result.DomainRecords.Record))
	for _, r := range result.DomainRecords.Record {
		// 客户端精确匹配 RR（API 的 RRKeyWord 是模糊匹配）
		if subDomain != "" && r.RR != subDomain {
			continue
		}
		records = append(records, providers.RecordInfo{
			ID:    r.RecordId,
			Name:  r.RR,
			Type:  r.Type,
			Value: r.Value,
			TTL:   r.TTL,
		})
	}
	return records, nil
}

// GetDomainRecord 查询单条解析记录详情
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
		log.Debug("probing Alibaba root domain", "domain", h)

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

		// TotalCount > 0 表示域名存在且有 DNS 记录，确认是有效根域名
		// TotalCount == 0 可能是域名不存在，或是存在但无记录
		// 无法区分两者，保守起见继续探测更深的域名段
		if result.TotalCount == 0 {
			continue
		}

		subDomain := strings.Join(parts[:i], ".")
		log.Info("Alibaba root domain found", "root", h, "subdomain", subDomain)
		return h, subDomain, nil
	}

	// 兜底：完整域名即为根域名，使用 @ 表示记录主机名
	return domain, "@", nil
}

// makeRequest performs an authenticated request to Alibaba Cloud API
func (c *AliDNSClient) makeRequest(ctx context.Context, params map[string]string) ([]byte, error) {
	action := params["Action"]
	log.Debug("Alibaba Cloud API request", "action", action)

	// 复制参数避免污染调用方 map
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

	// 参数排序
	var keys []string
	for k := range reqParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建查询字符串
	var queryParts []string
	for _, k := range keys {
		queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, url.QueryEscape(reqParams[k])))
	}
	queryString := strings.Join(queryParts, "&")

	// 计算签名
	stringToSign := fmt.Sprintf("GET&%%2F&%s", url.QueryEscape(queryString))
	mac := hmac.New(sha1.New, []byte(c.AccessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	signature = url.QueryEscape(signature)

	// Build final URL (注意：不记录完整 URL 以免泄露签名和 AccessKeyId)
	fullURL := fmt.Sprintf("%s?%s&Signature=%s", c.BaseURL, queryString, signature)

	// 发起请求
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		log.Error("Alibaba Cloud API request failed", "action", action, "err", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Debug("Alibaba Cloud API response", "action", action, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Error("Alibaba Cloud API returned error status",
			"action", action, "status", resp.StatusCode)
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
		log.Error("Alibaba Cloud API business error",
			"action", action, "message", apiError.Message)
		return nil, fmt.Errorf("API error: %s", apiError.Message)
	}

	return body, nil
}
