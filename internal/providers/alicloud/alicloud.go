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
	"maps"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
)

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
func (c *AliDNSClient) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	domain, subDomain, err := c.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	params := map[string]string{
		"Action":     "AddDomainRecord",
		"DomainName": domain,
		"RR":         subDomain,
		"Type":       record.Type,
		"Value":      record.Value,
		"TTL":        fmt.Sprintf("%d", record.TTL),
		"RecordLine": "default",
	}

	_, err = c.makeV1Request(ctx, params)
	return err
}

// ModifyRecord 修改域名解析记录
func (c *AliDNSClient) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	_, subDomain, err := c.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	params := map[string]string{
		"Action":   "UpdateDomainRecord",
		"RecordId": record.ID,
		"RR":       subDomain,
		"Type":     record.Type,
		"Value":    record.Value,
		"TTL":      fmt.Sprintf("%d", record.TTL),
	}

	_, err = c.makeV1Request(ctx, params)
	return err
}

// DeleteRecord 删除域名解析记录
func (c *AliDNSClient) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	params := map[string]string{
		"Action":   "DeleteDomainRecord",
		"RecordId": record.ID,
	}

	_, err := c.makeV1Request(ctx, params)
	return err
}

// GetRecords 查询域名的解析记录，返回通用 RecordInfo 列表
func (c *AliDNSClient) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	domain, subDomain, err := c.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}

	params := map[string]string{
		"Action":      "DescribeDomainRecords",
		"DomainName":  domain,
		"TypeKeyWord": recordType,
	}
	// 有明确的子域名时才传 RRKeyWord 过滤
	// subDomain 为 "@" 表示根域名（getRootDomain 的兜底返回值），
	// 此时不传 RRKeyWord 可获取该域名下所有记录
	if subDomain != "@" {
		params["RRKeyWord"] = subDomain
	}

	resp, err := c.makeV1Request(ctx, params)
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

	records := make([]ddns.RecordInfo, 0, len(result.DomainRecords.Record))
	for _, r := range result.DomainRecords.Record {
		// 客户端精确匹配 RR（API 的 RRKeyWord 是模糊匹配）
		// subDomain 为 "@" 时跳过此过滤以展示所有记录
		if subDomain != "@" && subDomain != "" && r.RR != subDomain {
			continue
		}
		records = append(records, ddns.RecordInfo{
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

	resp, err := c.makeV1Request(ctx, params)
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
		slog.Debug("probing Alibaba root domain", "module", "alicloud", "domain", h)

		params := map[string]string{
			"Action":     "DescribeDomainRecords",
			"DomainName": h,
		}

		resp, err := c.makeV1Request(ctx, params)
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
		slog.Info("Alibaba root domain found", "module", "alicloud", "root", h, "subdomain", subDomain)
		return h, subDomain, nil
	}

	// 兜底：完整域名即为根域名，使用 @ 表示记录主机名
	return domain, "@", nil
}

// makeV1Request 使用 V1 签名（HMAC-SHA1）发起认证请求。
//
// 签名方式：所有参数放入查询字符串，对整个查询字符串进行 HMAC-SHA1 签名，
// 签名结果附加在 URL 末尾。
func (c *AliDNSClient) makeV1Request(ctx context.Context, params map[string]string) ([]byte, error) {
	action := params["Action"]
	slog.Debug("Alibaba Cloud API request", "module", "alicloud", "action", action)

	// 复制参数避免污染调用方 map
	reqParams := make(map[string]string, len(params)+8)
	maps.Copy(reqParams, params)
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
		slog.Error("Alibaba Cloud API request failed", "module", "alicloud", "action", action, "err", err)
		return nil, err
	}
	defer resp.Body.Close()

	slog.Debug("Alibaba Cloud API response", "module", "alicloud", "action", action, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		slog.Debug("Alibaba Cloud API returned non-200 status",
			"module", "alicloud",
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
		slog.Error("Alibaba Cloud API business error",
			"module", "alicloud",
			"action", action, "message", apiError.Message)
		return nil, fmt.Errorf("API error: %s", apiError.Message)
	}

	return body, nil
}

// makeV3Request 使用 V3 签名（ACS3-HMAC-SHA256）发起认证请求。
//
// 签名方式：参数中的 Action 作为 x-acs-action 头，Version 作为 x-acs-version 头，
// 其余参数放入查询字符串，使用 HMAC-SHA256 对整个请求进行签名，
// 签名结果放在 Authorization 头中。
func (c *AliDNSClient) makeV3Request(ctx context.Context, params map[string]string) ([]byte, error) {
	action := params["Action"]
	if action == "" {
		return nil, fmt.Errorf("makeV3Request: missing 'Action' parameter")
	}
	slog.Debug("Alibaba Cloud API V3 request", "module", "alicloud", "action", action)

	// 从 BaseURL 解析主机名
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// 构建 V3 请求头
	headers := map[string]string{
		"x-acs-action":  action,
		"x-acs-version": "2015-01-09",
	}

	// 分离 Action 和 Version 参数，其余作为查询参数
	queryParams := make(map[string]string, len(params))
	for k, v := range params {
		switch k {
		case "Action":
		// 已在 headers 中设置
		case "Version":
			headers["x-acs-version"] = v
		default:
			queryParams[k] = v
		}
	}

	v3Req := &V3Request{
		AccessKeyId:     c.AccessKeyId,
		AccessKeySecret: c.AccessKeySecret,
		Method:          "GET",
		Scheme:          u.Scheme,
		Host:            u.Host,
		Path:            u.Path,
		QueryParams:     queryParams,
		Headers:         headers,
	}

	httpReq, err := SignV3(ctx, v3Req)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// 发起请求
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		slog.Error("Alibaba Cloud API V3 request failed", "module", "alicloud", "action", action, "err", err)
		return nil, err
	}
	defer resp.Body.Close()

	slog.Debug("Alibaba Cloud API V3 response", "module", "alicloud", "action", action, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Debug("Alibaba Cloud API V3 returned non-200 status",
			"module", "alicloud",
			"action", action, "status", resp.StatusCode, "body", truncateString(string(body), 200))
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 检查 API 业务错误
	var apiError struct {
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &apiError); err == nil && apiError.Message != "" {
		slog.Error("Alibaba Cloud API V3 business error",
			"module", "alicloud",
			"action", action, "message", apiError.Message)
		return nil, fmt.Errorf("API error: %s", apiError.Message)
	}

	return body, nil
}

// truncateString 截断字符串到指定长度，用于日志中避免泄露完整响应。
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
