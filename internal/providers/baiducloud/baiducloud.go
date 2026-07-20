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
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

const (
	defaultBaseURL = "https://bcd.baidubce.com"
)

// Client 百度云 DNS API 客户端
type Client struct {
	accessKey string
	secretKey string
	baseURL   string
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
	RecordID string `json:"recordId,omitempty"`
	Domain   string `json:"domain"`
	RDType   string `json:"rdtype"`
	TTL      int    `json:"ttl,omitempty"`
	RData    string `json:"rdata"`
	View     string `json:"view"`
	ZoneName string `json:"zoneName"`
}

// baiduListResponse 查询记录响应
type baiduListResponse struct {
	Result []struct {
		RecordID string `json:"recordId"`
		Domain   string `json:"domain"`
		RDType   string `json:"rdtype"`
		RData    string `json:"rdata"`
		TTL      int    `json:"ttl"`
		View     string `json:"view"`
		ZoneName string `json:"zoneName"`
	} `json:"result"`
	TotalCount int `json:"totalCount"`
}

// AddRecord 添加域名解析记录
func (c *Client) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	_, subDomain, zoneName := splitDomain(record.Name)

	payload := map[string]any{
		"domain":   subDomain,
		"rdType":   record.Type,
		"rdata":    record.Value,
		"ttl":      record.TTL,
		"zoneName": zoneName,
	}

	url := c.baseURL + "/v1/domain/resolve/add"
	slog.Debug("adding BaiduCloud DNS record", "module", "baiducloud", "zone", zoneName, "domain", subDomain, "type", record.Type)

	_, err := c.request(ctx, http.MethodPost, url, payload)
	if err != nil {
		return err
	}

	slog.Info("BaiduCloud DNS record added successfully", "module", "baiducloud", "zone", zoneName, "domain", subDomain, "ipv6", record.Value)
	return nil
}

// ModifyRecord 修改域名解析记录
func (c *Client) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	_, subDomain, zoneName := splitDomain(record.Name)

	// 查询现有记录获取 View 字段（解析线路）
	var recordView string
	// 直接调用百度云列表 API 获取现有记录
	listPayload := map[string]any{"domain": zoneName, "pageNum": 1, "pageSize": 1000}
	if raw, err := c.request(ctx, http.MethodPost, c.baseURL+"/v1/domain/resolve/list", listPayload); err == nil {
		var listResp baiduListResponse
		if json.Unmarshal(raw, &listResp) == nil {
			for _, r := range listResp.Result {
				if r.RecordID == record.ID {
					recordView = r.View
					break
				}
			}
		}
	}

	payload := map[string]any{
		"recordId": record.ID,
		"domain":   subDomain,
		"rdType":   record.Type,
		"rdata":    record.Value,
		"ttl":      record.TTL,
		"zoneName": zoneName,
		"view":     recordView,
	}

	url := c.baseURL + "/v1/domain/resolve/edit"
	slog.Debug("modifying BaiduCloud DNS record", "module", "baiducloud", "zone", zoneName, "record_id", record.ID)

	_, err := c.request(ctx, http.MethodPost, url, payload)
	if err != nil {
		return err
	}

	slog.Info("BaiduCloud DNS record modified successfully", "module", "baiducloud", "zone", zoneName, "record_id", record.ID, "ipv6", record.Value)
	return nil
}

// DeleteRecord 删除域名解析记录
// 百度云 DNS API 未直接提供删除记录的接口，暂不支持
func (c *Client) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	slog.Warn("BaiduCloud does not support deleting records via API, skipping",
		"module", "baiducloud",
		"domain", record.Name, "record_id", record.ID)
	return nil
}

// GetRecords 查询域名解析记录
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	_, subDomain, zoneName := splitDomain(fulldomain)

	payload := map[string]any{
		"domain":   zoneName,
		"pageNum":  1,
		"pageSize": 1000,
	}

	url := c.baseURL + "/v1/domain/resolve/list"
	slog.Debug("querying BaiduCloud DNS records", "module", "baiducloud", "zone", zoneName)

	resp, err := c.request(ctx, http.MethodPost, url, payload)
	if err != nil {
		return nil, err
	}

	var listResp baiduListResponse
	if err := json.Unmarshal(resp, &listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := make([]ddns.RecordInfo, 0, len(listResp.Result))
	for _, r := range listResp.Result {
		if r.RDType != recordType {
			continue
		}
		// 按子域名过滤
		if r.Domain != subDomain && subDomain != "@" {
			continue
		}
		result = append(result, ddns.RecordInfo{
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

// signRequest 为请求添加 BCE 认证签名
// 遵循百度云 BCE 签名规范：https://cloud.baidu.com/doc/Reference/s/Njwvz1wot
// 与 ddns-go 的 BaiduSigner 实现对对齐，不包含 body hash
func (c *Client) signRequest(req *http.Request, body []byte) {
	expirationSeconds := "1800"
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// BCE v1 签名前缀
	authStringPrefix := fmt.Sprintf("bce-auth-v1/%s/%s/%s", c.accessKey, timestamp, expirationSeconds)

	// CanonicalURI（全局路径）
	canonicalURI := req.URL.Path

	// CanonicalHeaders 和 SignedHeaders 硬编码为 host
	// 百度云 DNS API 只有一个端点 bcd.baidubce.com，所有操作都是 POST
	signedHeaders := "host"
	canonicalHeaders := "host:bcd.baidubce.com"
	canonicalQuery := ""

	// 构建 CanonicalRequest（不包含 body hash）
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s",
		req.Method, canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders)

	// 构建 StringToSign
	stringToSign := fmt.Sprintf("%s\n%s", authStringPrefix, sha256Hex([]byte(canonicalRequest)))

	// 计算签名
	signingKey := hmacSha256([]byte(c.secretKey), authStringPrefix)
	signature := hex.EncodeToString(hmacSha256(signingKey, stringToSign))

	// 设置认证头
	authorization := fmt.Sprintf("%s/%s/%s", authStringPrefix, signedHeaders, signature)
	req.Header.Set("Authorization", authorization)
}

// splitDomain 分割完整域名为子域名、根域名和 zone 名称
func splitDomain(fulldomain string) (string, string, string) {
	root, sub := domainutil.SplitDomain(fulldomain)
	return root, sub, root // zoneName 即根域名
}

// record.TypeToInt DNS 记录类型转百度云 RDType 数字

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
