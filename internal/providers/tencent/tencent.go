// tencent 实现 Tencent Cloud DNS 服务
package tencent

import (
	"bytes"
	"context"
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

const (
	service = "dnspod"
	version = "2021-03-23"
)

// DNSRecord 表示 Tencent Cloud DNS 记录
type DNSRecord struct {
	DomainId     int    `json:"DomainId,omitempty"`
	Domain       string `json:"Domain,omitempty"`
	SubDomain    string `json:"SubDomain,omitempty"`
	RecordId     string `json:"RecordId,omitempty"`
	RecordType   string `json:"RecordType,omitempty"`
	RecordLine   string `json:"RecordLine,omitempty"`
	RecordLineId string `json:"RecordLineId,omitempty"`
	Value        string `json:"Value,omitempty"`
	TTL          int    `json:"TTL,omitempty"`
}

// Response 表示 Tencent Cloud DNS API 响应
type Response struct {
	RecordId  string `json:"RecordId"`
	RequestId string `json:"RequestId"`
}

// DNSService 表示 Tencent Cloud DNS 服务
type DNSPod struct {
	secretId  string
	secretKey string
	apiURL    string
	*http.Client
}

// Option 表示 Tencent Cloud DNS 服务选项
type Option func(*DNSPod)

// Task 执行 Tencent Cloud DNS 更新任务
func (ds *DNSPod) Task(ctx context.Context, domain, subdomain, ipv6addr string) error {
	fulldomain := domain
	if subdomain != "@" {
		fulldomain = subdomain + "." + domain
	}

	record, err := ds.FindDomainRecord(ctx, fulldomain, "AAAA", ipv6addr)
	if err != nil {
		return fmt.Errorf("find AAAA record: %w", err)
	}
	if record != nil {
		return nil
	}

	records, err := ds.GetDomainRecords(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("get domain records: %w", err)
	}

	for _, r := range records {
		if r.RecordType == "AAAA" {
			return ds.ModifyDomainRecord(ctx, fulldomain, r.RecordId, "AAAA", ipv6addr, r.TTL)
		}
	}

	return ds.AddDomainRecord(ctx, fulldomain, "AAAA", ipv6addr, 600)
}

// NewDNSService 创建 Tencent Cloud DNS 服务实例
func NewDNSPod(secretId, secretKey string, options ...Option) *DNSPod {
	client := &DNSPod{
		secretId:  secretId,
		secretKey: secretKey,
		apiURL:    "https://dnspod.tencentcloudapi.com",
		Client:    &http.Client{Timeout: 30 * time.Second},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL 设置自定义基础 URL
func WithAPIUrl(url string) Option {
	return func(ds *DNSPod) {
		ds.apiURL = url
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(httpClient *http.Client) Option {
	return func(ds *DNSPod) {
		ds.Client = httpClient
	}
}

// AddDomainRecord 添加域名解析记录
func (ds *DNSPod) AddDomainRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	domain, subDomain, err := ds.getRootDomain(ctx, fulldomain)
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

	response := new(Response)
	return ds.makeRequest(ctx, "CreateRecord", record, response)
}

// ModifyDomainRecord 修改域名解析记录
func (ds *DNSPod) ModifyDomainRecord(ctx context.Context, fulldomain, recordId, recordType, newValue string, ttl int) error {
	domain, _, err := ds.getRootDomain(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := DNSRecord{
		Domain:     domain,
		RecordId:   recordId,
		RecordType: recordType,
		Value:      newValue,
		TTL:        ttl,
	}

	response := new(Response)
	return ds.makeRequest(ctx, "ModifyRecord", payload, response)
}

// DeleteDomainRecord 删除域名解析记录
func (ds *DNSPod) DeleteDomainRecord(ctx context.Context, fulldomain, recordId string) error {
	domain, _, err := ds.getRootDomain(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := DNSRecord{Domain: domain, RecordId: recordId}
	response := new(Response)
	return ds.makeRequest(ctx, "DeleteRecord", payload, response)
}

// GetDomainRecords 查询域名的所有解析记录
func (ds *DNSPod) GetDomainRecords(ctx context.Context, fulldomain string) ([]DNSRecord, error) {
	domain, subDomain, err := ds.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}
	return ds.describeRecords(ctx, domain, subDomain)
}

// GetDomainRecord 查询特定解析记录
func (ds *DNSPod) GetDomainRecord(ctx context.Context, fulldomain, recordId string) (*DNSRecord, error) {
	domain, _, err := ds.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := DNSRecord{Domain: domain, RecordId: recordId}

	var response struct {
		RecordInfo DNSRecord `json:"RecordInfo"`
	}

	err = ds.makeRequest(ctx, "DescribeRecord", payload, &response)
	if err != nil {
		return nil, err
	}

	return &response.RecordInfo, nil
}

// FindDomainRecord 根据子域名和值查找解析记录
func (ds *DNSPod) FindDomainRecord(ctx context.Context, fulldomain, recordType, value string) (*DNSRecord, error) {
	domain, subDomain, err := ds.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := DNSRecord{
		Domain:     domain,
		SubDomain:  subDomain,
		RecordType: recordType,
		Value:      value,
	}

	var response struct {
		RecordList []DNSRecord `json:"RecordList"`
	}

	err = ds.makeRequest(ctx, "DescribeRecordFilterList", payload, &response)
	if err != nil {
		return nil, err
	}

	if len(response.RecordList) > 0 {
		return &response.RecordList[0], nil
	}

	return nil, nil
}

// getRootDomain 从域名中提取根域名和子域名
func (ds *DNSPod) getRootDomain(ctx context.Context, domain string) (string, string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")

		// Check if this is a valid domain
		_, err := ds.describeRecords(ctx, h, "@")
		if err == nil {
			subDomain := strings.Join(parts[:i], ".")
			return h, subDomain, nil
		}
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// describeRecords 查询域名的所有解析记录
func (ds *DNSPod) describeRecords(ctx context.Context, domain, subDomain string) ([]DNSRecord, error) {
	payload := map[string]any{"Domain": domain, "Limit": 3000}

	var response struct {
		RecordList []DNSRecord `json:"RecordList"`
	}
	if err := ds.makeRequest(ctx, "DescribeRecordList", payload, &response); err != nil {
		return nil, err
	}

	recordList := response.RecordList

	// 过滤子域名记录
	if subDomain == "@" {
		return recordList, nil
	}

	for i := len(recordList) - 1; i >= 0; i-- {
		if recordList[i].SubDomain != subDomain {
			recordList = append(recordList[:i], recordList[i+1:]...)
		}
	}
	return recordList, nil
}

// makeRequest 执行Authenticated请求到Tencent Cloud API
func (ds *DNSPod) makeRequest(ctx context.Context, action string, payload any, result any) error {
	timestamp := time.Now().Unix()

	// 序列化请求体
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	// 生成签名
	signature := ds.generateSignatureV3(service, action, string(payloadBytes), timestamp)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", ds.apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", signature)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Action", action)

	// 发送请求
	resp, err := ds.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// 解析 Tencent Cloud API v3 外层响应
	var apiResponse struct {
		Response json.RawMessage `json:"Response"`
	}
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	// 检查 Response 内部的错误信息
	var errResp struct {
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	}
	if err := json.Unmarshal(apiResponse.Response, &errResp); err == nil && errResp.Error.Code != "" {
		return fmt.Errorf("API error: %s (%s)", errResp.Error.Message, errResp.Error.Code)
	}

	// 解析结果到传入的 result 指针
	if result != nil {
		if err := json.Unmarshal(apiResponse.Response, result); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}
	}

	return nil
}

// generateSignatureV3 生成Tencent Cloud API v3签名
func (ds *DNSPod) generateSignatureV3(service, action, payload string, timestamp int64) string {
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
	secretDate := hmacSha256("TC3"+ds.secretKey, date)
	secretService := hmacSha256Hex(string(secretDate), service)
	secretSigning := hmacSha256Hex(secretService, "tc3_request")
	signature := hmacSha256Hex(secretSigning, stringToSign)

	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, ds.secretId, credentialScope, signedHeaders, signature)
}

// sha256Hex 计算SHA256哈希值并返回十六进制字符串
func sha256Hex(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// hmacSha256 计算HMAC-SHA256哈希值
func hmacSha256(key, data string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// hmacSha256Hex 计算HMAC-SHA256哈希值并返回十六进制字符串
func hmacSha256Hex(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
