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

	"github.com/notes-bin/ddns6/internal/ddns"
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

// domainListItem DescribeDomainList 返回的域名列表项
type domainListItem struct {
	DomainId int    `json:"DomainId"`
	Name     string `json:"Name"`
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
		Client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				ForceAttemptHTTP2: false,
			},
		},
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
func (ds *DNSPod) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	log.Info("adding Tencent DNS record",
		"domain", record.Name, "type", record.Type)

	domain, subDomain, err := ds.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	dnsRecord := DNSRecord{
		Domain:     domain,
		SubDomain:  subDomain,
		RecordType: record.Type,
		RecordLine: "默认",
		Value:      record.Value,
		TTL:        record.TTL,
	}

	response := new(Response)
	err = ds.makeRequest(ctx, "CreateRecord", dnsRecord, response)
	if err != nil {
		log.Error("failed to add Tencent DNS record",
			"domain", record.Name, "type", record.Type, "err", err)
	}
	return err
}

// ModifyRecord 修改域名解析记录
func (ds *DNSPod) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	log.Info("modifying Tencent DNS record",
		"domain", record.Name, "record_id", record.ID, "type", record.Type)

	domain, subDomain, err := ds.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := DNSRecord{
		Domain:     domain,
		SubDomain:  subDomain,
		RecordId:   record.ID,
		RecordType: record.Type,
		RecordLine: "默认",
		Value:      record.Value,
		TTL:        record.TTL,
	}

	response := new(Response)
	err = ds.makeRequest(ctx, "ModifyRecord", payload, response)
	if err != nil {
		log.Error("failed to modify Tencent DNS record",
			"domain", record.Name, "record_id", record.ID, "err", err)
	}
	return err
}

// DeleteRecord 删除域名解析记录
func (ds *DNSPod) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	log.Info("deleting Tencent DNS record",
		"domain", record.Name, "record_id", record.ID)

	domain, _, err := ds.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	payload := DNSRecord{Domain: domain, RecordId: record.ID}
	response := new(Response)
	err = ds.makeRequest(ctx, "DeleteRecord", payload, response)
	if err != nil {
		log.Error("failed to delete Tencent DNS record",
			"domain", record.Name, "record_id", record.ID, "err", err)
	}
	return err
}

// GetRecords 查询域名的解析记录，返回通用 RecordInfo 列表
func (ds *DNSPod) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	domain, subDomain, err := ds.getRootDomain(ctx, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %v", err)
	}
	records, err := ds.describeRecords(ctx, domain, subDomain)
	if err != nil {
		return nil, err
	}

	result := make([]ddns.RecordInfo, 0, len(records))
	for _, r := range records {
		if recordType != "" && r.RecordType != recordType {
			continue
		}
		result = append(result, ddns.RecordInfo{
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
	// 优先通过 DescribeDomainList 获取域名列表，直接从列表匹配
	// 比逐一探测更可靠，且避免 InvalidParameter.DomainInvalid 问题
	domains, err := ds.getDomainList(ctx)
	if err == nil {
		for _, d := range domains {
			if d.Name == domain {
				log.Info("Tencent root domain found (exact match)", "root", domain)
				return domain, "@", nil
			}
			if strings.HasSuffix(domain, "."+d.Name) {
				root := d.Name
				subDomain := strings.TrimSuffix(domain, "."+root)
				log.Info("Tencent root domain found", "root", root, "subdomain", subDomain)
				return root, subDomain, nil
			}
		}

		// 域名不在列表中，给出明确提示
		domainNames := make([]string, 0, len(domains))
		for _, d := range domains {
			domainNames = append(domainNames, d.Name)
		}
		return "", "", fmt.Errorf("domain %q not found in account, available domains: %v; please add it in Tencent Cloud DNSPod console", domain, domainNames)
	}

	// DescribeDomainList 失败时回退到原有探测逻辑
	log.Warn("DescribeDomainList failed, falling back to probing", "err", err)
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")
		log.Debug("probing Tencent root domain", "domain", h)

		_, err := ds.describeRecords(ctx, h, "@")
		if err == nil {
			subDomain := strings.Join(parts[:i], ".")
			log.Info("Tencent root domain found (probe)", "root", h, "subdomain", subDomain)
			return h, subDomain, nil
		}
	}

	// 兜底：完整域名即为根域名
	_, err = ds.describeRecords(ctx, domain, "@")
	if err == nil {
		return domain, "@", nil
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// describeRecords 查询域名的所有解析记录
func (ds *DNSPod) describeRecords(ctx context.Context, domain, subDomain string) ([]DNSRecord, error) {
	log.Debug("querying Tencent DNS records", "domain", domain, "subdomain", subDomain)

	payload := map[string]any{"Domain": domain, "Limit": 3000}
	log.Debug("DescribeRecordList payload", "json", fmt.Sprintf("%v", payload))

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

// getDomainList 获取账户下所有域名列表（用于 getRootDomain 查询）
func (ds *DNSPod) getDomainList(ctx context.Context) ([]domainListItem, error) {
	payload := map[string]any{
		"Type":  "ALL",
		"Limit": 3000,
	}

	var resp struct {
		DomainList []domainListItem `json:"DomainList"`
	}
	if err := ds.makeRequest(ctx, "DescribeDomainList", payload, &resp); err != nil {
		return nil, err
	}

	return resp.DomainList, nil
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
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Host = req.URL.Host
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
// TC3-HMAC-SHA256 签名算法：
//
//	SecretDate    = HMAC-SHA256("TC3" + SecretKey, Date)
//	SecretService = HMAC-SHA256(SecretDate, Service)
//	SecretSigning = HMAC-SHA256(SecretService, "tc3_request")
//	Signature     = HexEncode(HMAC-SHA256(SecretSigning, StringToSign))
//
// 注意：中间密钥均为原始字节，仅在最后一步 HexEncode
func (ds *DNSPod) generateSignatureV3(service, action, payload string, timestamp int64) string {
	algorithm := "TC3-HMAC-SHA256"
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	domain := service + ".tencentcloudapi.com"

	// 构造规范请求
	canonicalURI := "/"
	canonicalQuery := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json; charset=utf-8\nhost:%s\nx-tc-action:%s\n", domain, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"
	hashedPayload := sha256Hex(payload)
	canonicalRequest := fmt.Sprintf("POST\n%s\n%s\n%s\n%s\n%s", canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, hashedPayload)

	// 构造待签名字符串
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedRequest := sha256Hex(canonicalRequest)
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s", algorithm, timestamp, credentialScope, hashedRequest)

	// 计算签名 — 密钥链全部使用原始字节，仅在最后一步 HexEncode
	secretDate := hmacSha256("TC3"+ds.secretKey, date)
	secretService := hmacSha256Bytes(secretDate, service)
	secretSigning := hmacSha256Bytes(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSha256Bytes(secretSigning, stringToSign))

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

// hmacSha256Bytes 计算HMAC-SHA256哈希值（密钥为原始字节）
func hmacSha256Bytes(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
