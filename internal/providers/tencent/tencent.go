// Package tencent 实现腾讯云 DNSPod API v3 服务。
//
// 认证方式：SecretID + SecretKey（TC3-HMAC-SHA256 签名）。
// 必填参数：--secret-id、--secret-key
//
// 与 internal/providers/dnspod（DNSPod 旧版 Token API）不同。
// 根域解析：有 zoneHint 直接拆分；否则优先 DescribeDomainList，失败再逐级探测。
// GetRecords：sub 为 "@" 时返回整区列表，否则按子域过滤；无 URL PathEscape（JSON POST）。
// 默认基于 httputil.NewHTTPClient，并自定义 Transport（保留 SameHostRedirect）。
package tencent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/internal/digest"
	"github.com/notes-bin/ddns6/internal/httputil"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

// 编译期断言：DNSPod 实现 ddns.DNSProvider。
var _ ddns.DNSProvider = (*DNSPod)(nil)

const (
	service           = "dnspod"
	version           = "2021-03-23"
	defaultRecordLine = "默认" // 默认 DNS 解析线路
)

// dnsRecord 表示腾讯云 DNS 记录。
// RecordID 在 API v20210323 中为数字类型。
//
// 请求（CreateRecord）使用 SubDomain/RecordType/RecordLine，
// 响应（DescribeRecordList/DescribeRecord）使用 Name/Type/Line/LineId；
// 通过 UnmarshalJSON 映射，使同一结构体可同时用于请求与响应。
type dnsRecord struct {
	DomainID     int    `json:"DomainId,omitzero"`
	Domain       string `json:"Domain,omitempty"`
	SubDomain    string `json:"SubDomain,omitempty"`
	RecordID     int    `json:"RecordId,omitzero"`
	RecordType   string `json:"RecordType,omitempty"`
	RecordLine   string `json:"RecordLine,omitempty"`
	RecordLineID string `json:"RecordLineId,omitempty"`
	Value        string `json:"Value,omitempty"`
	TTL          int    `json:"TTL,omitzero"`
}

// UnmarshalJSON 实现 json.Unmarshaler 接口。
//
// Tencent DNSPod API v20210323 的记录查询响应使用 Name/Type/Line/LineId 字段名，
// 而非创建/修改请求中的 SubDomain/RecordType/RecordLine/RecordLineId。
// 此方法读取响应中的 Name/Type/Line/LineId 值并映射到对应的 Go 字段，
// 确保同一结构体既能正确序列化请求，也能正确反序列化响应。
func (r *dnsRecord) UnmarshalJSON(data []byte) error {
	// 使用类型别名避免无限递归
	type Alias dnsRecord
	aux := &struct {
		*Alias
		Name   string `json:"Name"`
		Type   string `json:"Type"`
		Line   string `json:"Line"`
		LineID string `json:"LineId"`
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
	if r.RecordLineID == "" && aux.LineID != "" {
		r.RecordLineID = aux.LineID
	}

	return nil
}

// Response 为腾讯云 API 写操作的通用响应。
type Response struct {
	RecordID  int    `json:"RecordId"`
	RequestID string `json:"RequestId"`
}

// domainListItem 为 DescribeDomainList 返回的域名项。
type domainListItem struct {
	DomainID int    `json:"DomainId"`
	Name     string `json:"Name"`
}

// DNSPod 腾讯云 DNS API v3 客户端。
type DNSPod struct {
	secretID   string
	secretKey  string
	apiURL     string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*DNSPod)

// NewClient 创建腾讯云 DNSPod v3 客户端（httputil.NewHTTPClient + 自定义 Transport）。
func NewClient(secretID, secretKey string, options ...Option) *DNSPod {
	client := &DNSPod{
		secretID:  secretID,
		secretKey: secretKey,
		apiURL:    "https://dnspod.tencentcloudapi.com",
		httpClient: func() *http.Client {
			c := httputil.NewHTTPClient(30 * time.Second)
			c.Transport = &http.Transport{
				ForceAttemptHTTP2:   false,
				IdleConnTimeout:     90 * time.Second,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
			}
			return c
		}(),
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL 设置自定义 API 地址（测试用）。
func WithBaseURL(url string) Option {
	return func(ds *DNSPod) {
		ds.apiURL = strings.TrimSuffix(url, "/")
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(ds *DNSPod) {
		ds.httpClient = httpClient
	}
}

// AddRecord 添加 DNS 记录。
//
// 当 API 返回「记录已存在」时，查询现有记录：值相同则跳过，不同则改写，
// 以容忍服务重启或重复部署导致的配置重入。
func (ds *DNSPod) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	slog.Info("adding Tencent DNS record",
		"module", "tencent",
		"domain", record.Name, "type", record.Type)

	domain, subDomain, err := ds.getRootDomain(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %w", err)
	}

	dnsRecord := dnsRecord{
		Domain:     domain,
		SubDomain:  subDomain,
		RecordType: record.Type,
		RecordLine: defaultRecordLine,
		Value:      record.Value,
		TTL:        ddns.RecordTTL(record.TTL),
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

		records, qErr := ds.describeRecords(ctx, domain, subDomain)
		if qErr != nil {
			return fmt.Errorf("failed to query existing record after duplicate: %w", qErr)
		}

		wantSub := subDomain
		if wantSub == "" {
			wantSub = "@"
		}
		// 遍历匹配目标子域名与类型的记录
		for _, r := range records {
			if r.RecordType != record.Type {
				continue
			}
			got := r.SubDomain
			if got == "" {
				got = "@"
			}
			// 根域名记录可能返回 "@"、"" 或根域名本身
			match := strings.EqualFold(got, wantSub) ||
				(wantSub == "@" && (got == "@" || got == "" || strings.EqualFold(got, domain)))
			if !match {
				continue
			}
			if r.Value == record.Value {
				slog.Info("record already exists with same value, skipping",
					"module", "tencent", "domain", record.Name)
				return nil
			}
			slog.Info("record already exists with different value, updating",
				"module", "tencent",
				"domain", record.Name, "record_id", r.RecordID)
			return ds.ModifyRecord(ctx, ddns.RecordInfo{
				ID:    strconv.Itoa(r.RecordID),
				Name:  record.Name,
				Type:  record.Type,
				Value: record.Value,
				TTL:   ddns.RecordTTL(record.TTL),
			})
		}

		// 诊断：DescribeRecordList 返回了记录但未匹配到根域名 AAAA 记录
		if len(records) > 0 {
			for _, r := range records {
				slog.Warn("record detail for diagnosis",
					"module", "tencent",
					"record_id", r.RecordID,
					"sub_domain", r.SubDomain,
					"record_type", r.RecordType,
					"value", r.Value)
			}
		}
	}

	slog.Debug("failed to add Tencent DNS record",
		"module", "tencent",
		"domain", record.Name, "type", record.Type, "err", err)
	return err
}

// ModifyRecord 修改 DNS 记录。
func (ds *DNSPod) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	slog.Info("modifying Tencent DNS record",
		"module", "tencent",
		"domain", record.Name, "record_id", record.ID, "type", record.Type)

	domain, subDomain, err := ds.getRootDomain(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %w", err)
	}

	recordID, err := strconv.Atoi(record.ID)
	if err != nil {
		return fmt.Errorf("invalid record id %q: %w", record.ID, err)
	}

	payload := dnsRecord{
		Domain:     domain,
		SubDomain:  subDomain,
		RecordID:   recordID,
		RecordType: record.Type,
		RecordLine: defaultRecordLine,
		Value:      record.Value,
		TTL:        ddns.RecordTTL(record.TTL),
	}

	response := new(Response)
	err = ds.makeRequest(ctx, "ModifyRecord", payload, response)
	if err != nil {
		slog.Debug("failed to modify Tencent DNS record",
			"module", "tencent",
			"domain", record.Name, "record_id", record.ID, "err", err)
	}
	return err
}

// DeleteRecord 删除 DNS 记录。
func (ds *DNSPod) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	slog.Info("deleting Tencent DNS record",
		"module", "tencent",
		"domain", record.Name, "record_id", record.ID)

	domain, _, err := ds.getRootDomain(ctx, record.Name, record.Zone)
	if err != nil {
		return fmt.Errorf("failed to get root domain: %w", err)
	}

	recordID, err := strconv.Atoi(record.ID)
	if err != nil {
		return fmt.Errorf("invalid record id %q: %w", record.ID, err)
	}

	payload := dnsRecord{Domain: domain, RecordID: recordID}
	response := new(Response)
	err = ds.makeRequest(ctx, "DeleteRecord", payload, response)
	if err != nil {
		slog.Debug("failed to delete Tencent DNS record",
			"module", "tencent",
			"domain", record.Name, "record_id", record.ID, "err", err)
	}
	return err
}

// GetRecords 查询 DNS 记录。
// getRootDomain 解析后 DescribeRecordList：sub 为 "@" 返回整区，否则按子域过滤；
// Name 拼成 FQDN，便于后续 DeleteRecord 再拆根域。
func (ds *DNSPod) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	domain, subDomain, err := ds.getRootDomain(ctx, fulldomain, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %w", err)
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

		// 拼成含根域的完整 Name，供后续 DeleteRecord / getRootDomain 使用
		recordName := domain
		if r.SubDomain != "@" && r.SubDomain != "" {
			recordName = r.SubDomain + "." + domain
		}
		result = append(result, ddns.RecordInfo{
			ID:    strconv.Itoa(r.RecordID),
			Name:  recordName,
			Type:  r.RecordType,
			Value: r.Value,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// GetDomainRecord 查询单条 DNS 记录详情。
func (ds *DNSPod) GetDomainRecord(ctx context.Context, fulldomain, recordID string) (*dnsRecord, error) {
	domain, _, err := ds.getRootDomain(ctx, fulldomain, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get root domain: %w", err)
	}

	id, err := strconv.Atoi(recordID)
	if err != nil {
		return nil, fmt.Errorf("invalid record id %q: %w", recordID, err)
	}

	payload := dnsRecord{Domain: domain, RecordID: id}

	var response struct {
		RecordInfo dnsRecord `json:"RecordInfo"`
	}

	err = ds.makeRequest(ctx, "DescribeRecord", payload, &response)
	if err != nil {
		return nil, err
	}

	return &response.RecordInfo, nil
}

// getRootDomain 从完整域名解析账户内根域名与子域名。
// 有 zoneHint 时直接 SplitDomain，跳过 API；否则优先 DescribeDomainList，失败再逐级探测。
func (ds *DNSPod) getRootDomain(ctx context.Context, domain, zoneHint string) (string, string, error) {
	if zoneHint != "" {
		root, sub := domainutil.SplitDomain(domain, zoneHint)
		return root, sub, nil
	}

	// 优先账户域名列表匹配，避免无效 Domain 探测触发 DomainInvalid
	domains, err := ds.getDomainList(ctx)
	if err == nil {
		for _, d := range domains {
			if d.Name == domain {
				slog.Info("Tencent root domain found (exact match)", "module", "tencent", "root", domain)
				return domain, "@", nil
			}
			if subDomain, ok := strings.CutSuffix(domain, "."+d.Name); ok {
				root := d.Name
				slog.Info("Tencent root domain found", "module", "tencent", "root", root, "subdomain", subDomain)
				return root, subDomain, nil
			}
		}

		domainNames := make([]string, 0, len(domains))
		for _, d := range domains {
			domainNames = append(domainNames, d.Name)
		}
		return "", "", fmt.Errorf("domain %q not found in account, available domains: %v; please add it in tencent cloud dnspod console", domain, domainNames)
	}

	slog.Warn("DescribeDomainList failed, falling back to probing", "module", "tencent", "err", err)
	parts := strings.Split(domain, ".")
	for i := range len(parts) - 1 {
		h := strings.Join(parts[i+1:], ".")
		slog.Debug("probing Tencent root domain", "module", "tencent", "domain", h)

		_, err := ds.describeRecords(ctx, h, "@")
		if err == nil {
			subDomain := strings.Join(parts[:i+1], ".")
			slog.Info("Tencent root domain found (probe)", "module", "tencent", "root", h, "subdomain", subDomain)
			return h, subDomain, nil
		}
	}

	_, err = ds.describeRecords(ctx, domain, "@")
	if err == nil {
		return domain, "@", nil
	}

	return "", "", fmt.Errorf("could not find root domain for %s", domain)
}

// describeRecords 调用 DescribeRecordList；subDomain 非 "@" 时再按主机名本地过滤。
func (ds *DNSPod) describeRecords(ctx context.Context, domain, subDomain string) ([]dnsRecord, error) {
	slog.Debug("querying Tencent DNS records", "module", "tencent", "domain", domain, "subdomain", subDomain)

	payload := map[string]any{"Domain": domain, "Limit": 3000}
	slog.Debug("DescribeRecordList payload", "module", "tencent", "json", fmt.Sprintf("%v", payload))

	var response struct {
		RecordList []dnsRecord `json:"RecordList"`
	}
	if err := ds.makeRequest(ctx, "DescribeRecordList", payload, &response); err != nil {
		return nil, err
	}

	recordList := response.RecordList

	if subDomain == "@" {
		return recordList, nil
	}

	filtered := make([]dnsRecord, 0, len(recordList))
	for _, r := range recordList {
		if r.SubDomain == subDomain {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// getDomainList 获取账户下域名列表，供 getRootDomain 匹配。
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

// makeRequest 向腾讯云 API 发起已签名的 POST 请求并解码 Response。
func (ds *DNSPod) makeRequest(ctx context.Context, action string, payload any, result any) error {
	slog.Debug("Tencent API request", "module", "tencent", "action", action)

	timestamp := time.Now().Unix()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	signature := ds.generateSignatureV3(service, action, string(payloadBytes), timestamp)

	req, err := http.NewRequestWithContext(ctx, "POST", ds.apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Host = req.URL.Host
	req.Header.Set("Authorization", signature)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Action", action)

	resp, err := ds.httpClient.Do(req)
	if err != nil {
		slog.Debug("Tencent API request failed", "module", "tencent", "action", action, "err", err)
		return fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	// 先读完整 body，错误与成功路径共用，避免重复读取
	bodyBytes, err := httputil.ReadBody(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	slog.Debug("Tencent API response", "module", "tencent", "action", action, "status", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Debug("Tencent API returned error status",
			"module", "tencent",
			"action", action, "status", resp.StatusCode)
		return fmt.Errorf("api request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResponse struct {
		Response json.RawMessage `json:"Response"`
	}
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResponse.Response) == 0 || string(apiResponse.Response) == "null" {
		return fmt.Errorf("api returned null response for action %s", action)
	}

	var errResp struct {
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	}
	if err := json.Unmarshal(apiResponse.Response, &errResp); err == nil && errResp.Error.Code != "" {
		slog.Debug("Tencent API business error",
			"module", "tencent",
			"action", action, "code", errResp.Error.Code,
			"message", errResp.Error.Message)
		return fmt.Errorf("api error: %s (%s)", errResp.Error.Message, errResp.Error.Code)
	}

	if result != nil {
		if err := json.Unmarshal(apiResponse.Response, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// generateSignatureV3 生成腾讯云 API v3（TC3-HMAC-SHA256）Authorization 头。
//
//	SecretDate    = HMAC-SHA256("TC3" + SecretKey, Date)
//	SecretService = HMAC-SHA256(SecretDate, Service)
//	SecretSigning = HMAC-SHA256(SecretService, "tc3_request")
//	Signature     = HexEncode(HMAC-SHA256(SecretSigning, StringToSign))
//
// 中间密钥均为原始字节，仅最后一步 HexEncode。
func (ds *DNSPod) generateSignatureV3(service, action, payload string, timestamp int64) string {
	algorithm := "TC3-HMAC-SHA256"
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	domain := service + ".tencentcloudapi.com"

	canonicalURI := "/"
	canonicalQuery := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json; charset=utf-8\nhost:%s\nx-tc-action:%s\n", domain, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"
	hashedPayload := digest.SHA256Hex([]byte(payload))
	canonicalRequest := fmt.Sprintf("POST\n%s\n%s\n%s\n%s\n%s", canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, hashedPayload)

	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedRequest := digest.SHA256Hex([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s", algorithm, timestamp, credentialScope, hashedRequest)

	secretDate := digest.HMACSHA256([]byte("TC3"+ds.secretKey), []byte(date))
	secretService := digest.HMACSHA256(secretDate, []byte(service))
	secretSigning := digest.HMACSHA256(secretService, []byte("tc3_request"))
	signature := digest.HMACSHA256Hex(secretSigning, []byte(stringToSign))

	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, ds.secretID, credentialScope, signedHeaders, signature)
}
