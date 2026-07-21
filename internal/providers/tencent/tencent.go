// tencent 实现 Tencent Cloud DNS 服务
package tencent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/crypto"
	"github.com/notes-bin/ddns6/internal/ddns"
)

const (
	service = "dnspod"
	version = "2021-03-23"
)

// DNSRecord 表示 Tencent Cloud DNS 记录
// 注意：Tencent API v20210323 中 RecordId 为数字类型
//
// 重要：API 请求（CreateRecord）使用 SubDomain/RecordType/RecordLine 字段名，
// 但响应（DescribeRecordList/DescribeRecord）使用 Name/Type/Line/LineId 字段名。
// 自定义 UnmarshalJSON 处理此差异，保证 Go 结构体字段名不变的同时正确解析响应。
type DNSRecord struct {
	DomainId     int    `json:"DomainId,omitempty"`
	Domain       string `json:"Domain,omitempty"`
	SubDomain    string `json:"SubDomain,omitempty"`
	RecordId     int    `json:"RecordId,omitempty"`
	RecordType   string `json:"RecordType,omitempty"`
	RecordLine   string `json:"RecordLine,omitempty"`
	RecordLineId string `json:"RecordLineId,omitempty"`
	Value        string `json:"Value,omitempty"`
	TTL          int    `json:"TTL,omitempty"`
}

// UnmarshalJSON 实现 json.Unmarshaler 接口。
//
// Tencent DNSPod API v20210323 的记录查询响应使用 Name/Type/Line/LineId 字段名，
// 而非创建/修改请求中的 SubDomain/RecordType/RecordLine/RecordLineId。
// 此方法读取响应中的 Name/Type/Line/LineId 值并映射到对应的 Go 字段，
// 确保同一结构体既能正确序列化请求，也能正确反序列化响应。
func (r *DNSRecord) UnmarshalJSON(data []byte) error {
	// 使用类型别名避免无限递归
	type Alias DNSRecord
	aux := &struct {
		*Alias
		Name   string `json:"Name"`
		Type   string `json:"Type"`
		Line   string `json:"Line"`
		LineId string `json:"LineId"`
	}{Alias: (*Alias)(r)}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// 将 API 返回的字段名映射到 Go 结构体字段
	// 仅当标准解析未设置时进行映射，兼容两种命名风格
	if r.SubDomain == "" && aux.Name != "" {
		r.SubDomain = aux.Name
	}
	if r.RecordType == "" && aux.Type != "" {
		r.RecordType = aux.Type
	}
	if r.RecordLine == "" && aux.Line != "" {
		r.RecordLine = aux.Line
	}
	if r.RecordLineId == "" && aux.LineId != "" {
		r.RecordLineId = aux.LineId
	}

	return nil
}

// Response API 响应
type Response struct {
	RecordId  int    `json:"RecordId"`
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

// AddRecord 添加域名解析记录。
//
// 当 Tencent API 返回「记录已存在」错误时，会查询现有记录并执行修改（值不同时）
// 或跳过（值相同时），使运行中能容忍配置重入（service restart / re-deploy）。
func (ds *DNSPod) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	slog.Info("adding Tencent DNS record",
		"module", "tencent",
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
	if err == nil {
		return nil
	}

	// 记录已存在：查询现有记录并判断是否需要修改。
	//
	// Tencent DNSPod API 对不同场景返回的 SubDomain 值可能不统一
	//（"@"、""、"notes-bin.top" 等），因此不依赖 SubDomain 精确匹配，
	// 而是按记录类型 + 根域名特性模糊匹配。
	if strings.Contains(err.Error(), "InvalidParameter.DomainRecordExist") {
		slog.Debug("record already exists, querying existing records",
			"module", "tencent", "domain", domain, "type", record.Type)

		records, qErr := ds.describeRecords(ctx, domain, "@")
		if qErr != nil {
			return fmt.Errorf("failed to query existing record after duplicate: %w", qErr)
		}

		// 遍历所有记录，查找匹配目标类型的记录。
		// 不严格限制 SubDomain 值，兼容 Tencent API 的不同返回格式。
		// 根域名记录 SubDomain 可能为 "@"、"" 或根域名本身（如 "notes-bin.top"）。
		for _, r := range records {
			if r.RecordType != record.Type {
				continue
			}
			// 排除非根域名的子域名记录（如 www、mail 等），
			// 防止在存在多个 AAAA 记录时修改错误的记录
			if r.SubDomain != "@" && r.SubDomain != "" && r.SubDomain != domain {
				continue
			}
			if r.Value == record.Value {
				slog.Info("record already exists with same value, skipping",
					"module", "tencent", "domain", record.Name)
				return nil
			}
			slog.Info("record already exists with different value, updating",
				"module", "tencent",
				"domain", record.Name, "record_id", r.RecordId)
			return ds.ModifyRecord(ctx, ddns.RecordInfo{
				ID:    strconv.Itoa(r.RecordId),
				Name:  record.Name,
				Type:  record.Type,
				Value: record.Value,
				TTL:   record.TTL,
			})
		}

		// 诊断：DescribeRecordList 返回了记录但未匹配到根域名 AAAA 记录
		if len(records) > 0 {
			for _, r := range records {
				slog.Warn("record detail for diagnosis",
					"module", "tencent",
					"record_id", r.RecordId,
					"sub_domain", r.SubDomain,
					"record_type", r.RecordType,
					"value", r.Value)
			}
		}
	}

	slog.Error("failed to add Tencent DNS record",
		"module", "tencent",
		"domain", record.Name, "type", record.Type, "err", err)
	return err
}

// ModifyRecord 修改域名解析记录
func (ds *DNSPod) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	slog.Info("modifying Tencent DNS record",
		"module", "tencent",
		"domain", record.Name, "record_id", record.ID, "type", record.Type)

	domain, subDomain, err := ds.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	recordId, err := strconv.Atoi(record.ID)
	if err != nil {
		return fmt.Errorf("invalid record ID %q: %v", record.ID, err)
	}

		payload := DNSRecord{
			Domain:     domain,
			SubDomain:  subDomain,
			RecordId:   recordId,
			RecordType: record.Type,
		RecordLine: "默认",
		Value:      record.Value,
		TTL:        record.TTL,
	}

	response := new(Response)
	err = ds.makeRequest(ctx, "ModifyRecord", payload, response)
	if err != nil {
		slog.Error("failed to modify Tencent DNS record",
			"module", "tencent",
			"domain", record.Name, "record_id", record.ID, "err", err)
	}
	return err
}

// DeleteRecord 删除域名解析记录
func (ds *DNSPod) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	slog.Info("deleting Tencent DNS record",
		"module", "tencent",
		"domain", record.Name, "record_id", record.ID)

	domain, _, err := ds.getRootDomain(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %v", err)
	}

	recordId, err := strconv.Atoi(record.ID)
	if err != nil {
		return fmt.Errorf("invalid record ID %q: %v", record.ID, err)
	}

	payload := DNSRecord{Domain: domain, RecordId: recordId}
	response := new(Response)
	err = ds.makeRequest(ctx, "DeleteRecord", payload, response)
	if err != nil {
		slog.Error("failed to delete Tencent DNS record",
			"module", "tencent",
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

		// 构建完整记录名（含根域名），而非仅 API 返回的标签。
		// 这样 DeleteRecord 等后续操作能正确提取根域名。
		recordName := domain
		if r.SubDomain != "@" && r.SubDomain != "" {
			recordName = r.SubDomain + "." + domain
		}
		result = append(result, ddns.RecordInfo{
			ID:    strconv.Itoa(r.RecordId),
			Name:  recordName,
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

	recordID, err := strconv.Atoi(recordId)
	if err != nil {
		return nil, fmt.Errorf("invalid record ID %q: %v", recordId, err)
	}

	payload := DNSRecord{Domain: domain, RecordId: recordID}

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
				slog.Info("Tencent root domain found (exact match)", "module", "tencent", "root", domain)
				return domain, "@", nil
			}
			if strings.HasSuffix(domain, "."+d.Name) {
				root := d.Name
				subDomain := strings.TrimSuffix(domain, "."+root)
				slog.Info("Tencent root domain found", "module", "tencent", "root", root, "subdomain", subDomain)
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
	slog.Warn("DescribeDomainList failed, falling back to probing", "module", "tencent", "err", err)
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")
		slog.Debug("probing Tencent root domain", "module", "tencent", "domain", h)

		_, err := ds.describeRecords(ctx, h, "@")
		if err == nil {
			subDomain := strings.Join(parts[:i], ".")
			slog.Info("Tencent root domain found (probe)", "module", "tencent", "root", h, "subdomain", subDomain)
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
	slog.Debug("querying Tencent DNS records", "module", "tencent", "domain", domain, "subdomain", subDomain)

	payload := map[string]any{"Domain": domain, "Limit": 3000}
	slog.Debug("DescribeRecordList payload", "module", "tencent", "json", fmt.Sprintf("%v", payload))

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
	err := ds.makeRequest(ctx, "DescribeDomainList", payload, &resp)
	if err != nil {
		slog.Warn("DescribeDomainList failed, will fall back to domain probing", "module", "tencent", "err", err)
		return nil, err
	}

	// 记录账户下的域名列表
	if len(resp.DomainList) == 0 {
		slog.Warn("DescribeDomainList returned empty domain list", "module", "tencent")
	} else {
		names := make([]string, 0, len(resp.DomainList))
		for _, d := range resp.DomainList {
			names = append(names, d.Name)
		}
		slog.Info("DescribeDomainList - domains in account", "module", "tencent", "domains", names)
	}

	return resp.DomainList, nil
}

// makeRequest 执行Authenticated请求到Tencent Cloud API
func (ds *DNSPod) makeRequest(ctx context.Context, action string, payload any, result any) error {
	slog.Debug("Tencent API request", "module", "tencent", "action", action)

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
		slog.Error("Tencent API request failed", "module", "tencent", "action", action, "err", err)
		return fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	// 读取完整响应体（提前读取，后续直接使用 bodyBytes 避免重复 ReadAll）
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	slog.Debug("Tencent API response", "module", "tencent", "action", action, "status", resp.StatusCode)

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("Tencent API returned error status",
			"module", "tencent",
			"action", action, "status", resp.StatusCode)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
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
		slog.Error("Tencent API business error",
			"module", "tencent",
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
	hashedPayload := crypto.SHA256Hex([]byte(payload))
	canonicalRequest := fmt.Sprintf("POST\n%s\n%s\n%s\n%s\n%s", canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, hashedPayload)

	// 构造待签名字符串
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedRequest := crypto.SHA256Hex([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s", algorithm, timestamp, credentialScope, hashedRequest)

	// 计算签名 — 密钥链全部使用原始字节，仅在最后一步 HexEncode
	secretDate := crypto.HMACSHA256([]byte("TC3"+ds.secretKey), []byte(date))
	secretService := crypto.HMACSHA256(secretDate, []byte(service))
	secretSigning := crypto.HMACSHA256(secretService, []byte("tc3_request"))
	signature := crypto.HMACSHA256Hex(secretSigning, []byte(stringToSign))

	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, ds.secretId, credentialScope, signedHeaders, signature)
}

