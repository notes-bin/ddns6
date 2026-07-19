// Package baiducloud 实现百度云 DNS API 服务
// 百度云 DNS 使用 BCE（Baidu Cloud Engine）认证协议，基于 HMAC-SHA256 签名
package baiducloud

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
	"sort"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/providers"
)

var log = slog.With("module", "baiducloud")

const (
	defaultBaseURL = "https://bcd.baidubce.com"
)

// Client 百度云 DNS API 客户端
type Client struct {
	accessKey    string
	secretKey    string
	baseURL      string
	*http.Client
}

// Option 客户端配置选项函数
type Option func(*Client)

// NewClient 创建百度云 DNS 客户端
func NewClient(accessKey, secretKey string, options ...Option) *Client {
	c := &Client{
		accessKey: accessKey,
		secretKey: secretKey,
		baseURL:   defaultBaseURL,
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

// WithBaseURL 设置自定义 API 地址（测试用）
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.Client = httpClient
	}
}

// DNSRecord 百度云 DNS 记录
type DNSRecord struct {
	RecordID   string `json:"recordId,omitempty"`
	Domain     string `json:"domain"`
	RDType     int    `json:"rdtype"`
	TTL        int    `json:"ttl,omitempty"`
	RData      string `json:"rdata"`
	ZoneName   string `json:"zoneName"`
}

// baiduListResponse 查询记录响应
type baiduListResponse struct {
	Result []struct {
		RecordID   string `json:"recordId"`
		Domain     string `json:"domain"`
		RDType     int    `json:"rdtype"`
		RData      string `json:"rdata"`
		TTL        int    `json:"ttl"`
		ZoneName   string `json:"zoneName"`
	} `json:"result"`
	TotalCount int `json:"totalCount"`
}

// AddRecord 添加域名解析记录
func (c *Client) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	_, subDomain, zoneName := splitDomain(fulldomain)
	rdType := recordTypeToInt(recordType)

	payload := map[string]any{
		"domain":   subDomain,
		"rdType":   rdType,
		"rdata":    value,
		"ttl":      ttl,
		"zoneName": zoneName,
	}

	url := c.baseURL + "/v1/domain/resolve/add"
	log.Debug("adding BaiduCloud DNS record", "zone", zoneName, "domain", subDomain, "type", recordType)

	_, err := c.request(ctx, http.MethodPost, url, payload)
	if err != nil {
		return err
	}

	log.Info("BaiduCloud DNS record added successfully", "zone", zoneName, "domain", subDomain, "ipv6", value)
	return nil
}

// ModifyRecord 修改域名解析记录
func (c *Client) ModifyRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	_, subDomain, zoneName := splitDomain(fulldomain)
	rdType := recordTypeToInt(recordType)

	payload := map[string]any{
		"recordId": recordID,
		"domain":   subDomain,
		"rdType":   rdType,
		"rdata":    newValue,
		"ttl":      ttl,
		"zoneName": zoneName,
	}

	url := c.baseURL + "/v1/domain/resolve/edit"
	log.Debug("modifying BaiduCloud DNS record", "zone", zoneName, "record_id", recordID)

	_, err := c.request(ctx, http.MethodPost, url, payload)
	if err != nil {
		return err
	}

	log.Info("BaiduCloud DNS record modified successfully", "zone", zoneName, "record_id", recordID, "ipv6", newValue)
	return nil
}

// DeleteRecord 删除域名解析记录
// 百度云 DNS API 未直接提供删除记录的接口，暂不支持
func (c *Client) DeleteRecord(ctx context.Context, fulldomain, recordID string) error {
	log.Warn("BaiduCloud does not support deleting records via API, skipping",
		"domain", fulldomain, "record_id", recordID)
	return nil
}

// GetRecords 查询域名解析记录
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
	_, subDomain, zoneName := splitDomain(fulldomain)
	rdType := recordTypeToInt(recordType)

	payload := map[string]any{
		"domain":   zoneName,
		"pageNum":  1,
		"pageSize": 1000,
	}

	url := c.baseURL + "/v1/domain/resolve/list"
	log.Debug("querying BaiduCloud DNS records", "zone", zoneName)

		resp, err := c.request(ctx, http.MethodPost, url, payload)
	if err != nil {
		return nil, err
	}

	var listResp baiduListResponse
	if err := json.Unmarshal(resp, &listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := make([]providers.RecordInfo, 0, len(listResp.Result))
	for _, r := range listResp.Result {
		if r.RDType != rdType {
			continue
		}
		// 按子域名过滤
		if r.Domain != subDomain && subDomain != "@" {
			continue
		}
		result = append(result, providers.RecordInfo{
			ID:    r.RecordID,
			Name:  r.Domain,
			Type:  recordType,
			Value: r.RData,
			TTL:   r.TTL,
		})
	}
	return result, nil
}

// request 执行签名 POST 请求
func (c *Client) request(ctx context.Context, method, url string, payload any) ([]byte, error) {
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 生成 BCE 签名
	c.signRequest(req, bodyBytes)

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("BaiduCloud API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("BaiduCloud API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// signRequest 为请求添加 BCE 认证签名
func (c *Client) signRequest(req *http.Request, body []byte) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	expirationSeconds := "1800"

	// 规范请求头
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}

	signedHeaders := []string{"host", "x-bce-date"}
	headers := map[string]string{
		"host":       host,
		"x-bce-date": timestamp,
	}

	// 构建 CanonicalRequest
	canonicalURI := req.URL.Path
	canonicalQuery := req.URL.RawQuery

	// 规范请求头（排序后）
	sort.Strings(signedHeaders)
	var canonicalHeaders strings.Builder
	for _, h := range signedHeaders {
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", h, strings.TrimSpace(headers[h])))
	}

	// 计算 body 的 SHA256
	bodyHash := sha256Hex(body)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		bodyHash)

	// 构建 StringToSign
	authStringPrefix := fmt.Sprintf("bce-auth-v1/%s/%s/%s", c.accessKey, timestamp, expirationSeconds)
	stringToSign := fmt.Sprintf("%s\n%s", authStringPrefix, sha256Hex([]byte(canonicalRequest)))

	// 计算签名
	signingKey := hmacSha256([]byte(c.secretKey), authStringPrefix)
	signature := hex.EncodeToString(hmacSha256(signingKey, stringToSign))

	// 设置认证头
	authorization := fmt.Sprintf("%s/SignedHeaders=%s/Signature=%s",
		authStringPrefix, strings.Join(signedHeaders, ";"), signature)

	req.Header.Set("Authorization", authorization)
	req.Header.Set("x-bce-date", timestamp)
}

// splitDomain 分割完整域名为子域名、根域名和 zone 名称
func splitDomain(fulldomain string) (string, string, string) {
	root, sub := providers.SplitDomain(fulldomain)
	return root, sub, root // zoneName 即根域名
}

// recordTypeToInt DNS 记录类型转百度云 RDType 数字
func recordTypeToInt(recordType string) int {
	switch recordType {
	case "A":
		return 1
	case "AAAA":
		return 28
	case "CNAME":
		return 5
	case "MX":
		return 15
	case "TXT":
		return 16
	case "NS":
		return 2
	case "SRV":
		return 33
	default:
		return 28 // 默认 AAAA
	}
}

// sha256Hex 计算 SHA256 哈希并返回十六进制字符串
func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// hmacSha256 计算 HMAC-SHA256
func hmacSha256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
