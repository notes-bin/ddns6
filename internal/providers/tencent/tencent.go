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
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/providers"
)

var log = slog.With("module", "tencent")

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

// Response API 响应
type Response struct {
	RecordId  string `json:"RecordId"`
	RequestId string `json:"RequestId"`
}

// DNSPod Tencent Cloud DNS 服务客户端
type DNSPod struct {
	secretId  string
	secretKey string
	apiURL    string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*DNSPod)

// NewDNSPod 创建 Tencent Cloud DNS 服务实例
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

// WithAPIUrl 设置自定义 API 地址
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


// AddRecord 添加域名解析记录
func (ds *DNSPod) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	log.Info("adding Tencent DNS record",
		"domain", fulldomain, "type", recordType)

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
	err = ds.makeRequest(ctx, "CreateRecord", record, response)
	if err != nil {
		log.Error("failed to add Tencent DNS record",
			"domain", fulldomain, "type", recordType, "err", err)
	}
	return err
}

// ModifyRecord 修改域名解析记录
func (ds *DNSPod) ModifyRecord(ctx context.Context, fulldomain, recordId, recordType, newValue string, ttl int) error {
	log.Info("modifying Tencent DNS record",
		"domain", fulldomain, "record_id", recordId, "type", recordType)

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
	err = ds.makeRequest(ctx, "ModifyRecord", payload, response)
	if err != nil {
		log.Error("failed to modify Tencent DNS record",
			"domain", fulldomain, "record_id", recordId, "err", err)
	}
	return err
}

// DeleteRecord 删除域名解析记录
func (ds *DNSPod) DeleteRecord(ctx context.Context, fulldomain, recordId string) error {
	log.Info("deleting Tencent DNS record",
		"domain", fulldomain, "record_id", recordId)

	domain, _, err := ds.getRootDomain(ctx, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := DNSRecord{Domain: domain, RecordId: recordId}
	response := new(Response)
	err = ds.makeRequest(ctx, "DeleteRecord", payload, response)
	if err != nil {
		log.Error("failed to delete Tencent DNS record",
			"domain", fulldomain, "record_id", recordId, "err", err)
	}
	return err
}

// GetRecords 查询域名的解析记录，返回通用 RecordInfo 列表
func (ds *DNSPod) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
	domain, subDomain, err := ds.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}
	records, err := ds.describeRecords(ctx, domain, subDomain)
	if err != nil {
		return nil, err
	}

	result := make([]providers.RecordInfo, 0, len(records))
	for _, r := range records {
		if recordType != "" && r.RecordType != recordType {
			continue
		}
		result = append(result, providers.RecordInfo{
			ID:    r.RecordId,
			Name:  r.SubDomain,
			Type:  r.RecordType,
			Value: r.Value,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// GetDomainRecord 查询单条解析记录详情
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

// getRootDomain 从域名中提取根域名和子域名
func (ds *DNSPod) getRootDomain(ctx context.Context, domain string) (string, string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")
		log.Debug("probing Tencent root domain", "domain", h)

		// 验证是否为有效域名
		_, err := ds.describeRecords(ctx, h, "@")
		if err == nil {
			subDomain := strings.Join(parts[:i], ".")
			log.Info("Tencent root domain found", "root", h, "subdomain", subDomain)
			return h, subDomain, nil
		}
	}

	// 兜底：完整域名即为根域名，使用 @ 表示记录主机名
	_, err := ds.describeRecords(ctx, domain, "@")
	if err == nil {
		return domain, "@", nil
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// describeRecords 查询域名的所有解析记录
func (ds *DNSPod) describeRecords(ctx context.Context, domain, subDomain string) ([]DNSRecord, error) {
	log.Debug("querying Tencent DNS records", "domain", domain, "subdomain", subDomain)

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

	filtered := make([]DNSRecord, 0, len(recordList))
	for _, r := range recordList {
		if r.SubDomain == subDomain {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// makeRequest 执行Authenticated请求到Tencent Cloud API
func (ds *DNSPod) makeRequest(ctx context.Context, action string, payload any, result any) error {
	log.Debug("Tencent API request", "action", action)

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
		log.Error("Tencent API request failed", "action", action, "err", err)
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	log.Debug("Tencent API response", "action", action, "status", resp.StatusCode)

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Error("Tencent API returned error status",
			"action", action, "status", resp.StatusCode)
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

	// 处理 Response 为 null 的情况
	if len(apiResponse.Response) == 0 || string(apiResponse.Response) == "null" {
		return fmt.Errorf("API returned null Response for action %s", action)
	}

	// 检查 Response 内部的错误信息
	var errResp struct {
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	}
	if err := json.Unmarshal(apiResponse.Response, &errResp); err == nil && errResp.Error.Code != "" {
		log.Error("Tencent API business error",
			"action", action, "code", errResp.Error.Code,
			"message", errResp.Error.Message)
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

	// 构造规范请求
	canonicalURI := "/"
	canonicalQuery := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json\nhost:%s\nx-tc-action:%s\n", domain, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"
	hashedPayload := sha256Hex(payload)
	canonicalRequest := fmt.Sprintf("POST\n%s\n%s\n%s\n%s\n%s", canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, hashedPayload)

	// 构造待签名字符串
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedRequest := sha256Hex(canonicalRequest)
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s", algorithm, timestamp, credentialScope, hashedRequest)

	// 计算签名
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
