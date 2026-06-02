package huaweicloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/notes-bin/ddns6/internal/providers"
)

// HuaweiCloudClient 华为云 DNS API 客户端
type HuaweiCloudClient struct {
	Username   string
	Password   string
	DomainName string
	IAMURL     string
	DNSURL     string
	HTTPClient *http.Client

	tokenMu     sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

type Options func(*HuaweiCloudClient)

// NewClient 创建华为云 DNS 客户端
func NewClient(username, password, domainName string, options ...Options) *HuaweiCloudClient {
	client := &HuaweiCloudClient{
		Username:   username,
		Password:   password,
		DomainName: domainName,
		IAMURL:     "https://iam.myhuaweicloud.com",
		DNSURL:     "https://dns.ap-southeast-1.myhuaweicloud.com",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithIAMURL 设置自定义 IAM 地址
func WithIAMURL(url string) Options {
	return func(c *HuaweiCloudClient) {
		c.IAMURL = url
	}
}

// WithDNSURL 设置自定义 DNS API 地址
func WithDNSURL(url string) Options {
	return func(c *HuaweiCloudClient) {
		c.DNSURL = url
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
func WithHTTPClient(httpClient *http.Client) Options {
	return func(c *HuaweiCloudClient) {
		c.HTTPClient = httpClient
	}
}


// DNSRecord  a Huawei Cloud DNS record
type DNSRecord struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	TTL         int      `json:"ttl,omitempty"`
	Records     []string `json:"records"`
}

// AddRecord 添加域名解析记录
func (c *HuaweiCloudClient) AddRecord(ctx context.Context, fulldomain, recordType, value string, ttl int) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(ctx, token, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	record := DNSRecord{
		Name:    fulldomain + ".",
		Type:    recordType,
		TTL:     ttl,
		Records: []string{value},
	}

	_, err = c.createRecordSet(ctx, token, zoneID, record)
	return err
}

// ModifyRecord 修改域名解析记录
func (c *HuaweiCloudClient) ModifyRecord(ctx context.Context, fulldomain, recordID, recordType, newValue string, ttl int) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(ctx, token, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	record := DNSRecord{
		Name:    fulldomain + ".",
		Type:    recordType,
		TTL:     ttl,
		Records: []string{newValue},
	}

	_, err = c.updateRecordSet(ctx, token, zoneID, recordID, record)
	return err
}

// DeleteRecord 删除域名解析记录
func (c *HuaweiCloudClient) DeleteRecord(ctx context.Context, fulldomain, recordID string) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(ctx, token, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	return c.deleteRecordSet(ctx, token, zoneID, recordID)
}

// GetRecords 查询域名的解析记录，返回通用 RecordInfo 列表
func (c *HuaweiCloudClient) GetRecords(ctx context.Context, fulldomain, recordType string) ([]providers.RecordInfo, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(ctx, token, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %v", err)
	}

	// 华为云 DNS API 分页获取全部 recordsets，同时按名称和类型过滤
	var allRecordsets []DNSRecord
	marker := ""
	fqdn := fulldomain + "."
	for {
		reqURL := fmt.Sprintf("%s/v2/zones/%s/recordsets?limit=500&type=%s&name=%s",
			c.DNSURL, zoneID, recordType, fqdn)
		if marker != "" {
			reqURL += "&marker=" + url.QueryEscape(marker)
		}
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Auth-Token", token)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get records, status: %d", resp.StatusCode)
		}

		var apiResult struct {
			Recordsets []DNSRecord `json:"recordsets"`
			Links      struct {
				Next string `json:"next"`
			} `json:"links"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		allRecordsets = append(allRecordsets, apiResult.Recordsets...)

		if apiResult.Links.Next == "" {
			break
		}
		// 从 next link 中提取 marker
		if u, err := url.Parse(apiResult.Links.Next); err == nil {
			marker = u.Query().Get("marker")
		}
		if marker == "" {
			break
		}
	}

	var records []providers.RecordInfo
	for _, r := range allRecordsets {
		value := ""
		if len(r.Records) > 0 {
			value = r.Records[0]
		}
		records = append(records, providers.RecordInfo{
			ID:    r.ID,
			Name:  r.Name,
			Type:  r.Type,
			Value: value,
			TTL:   r.TTL,
		})
	}
	return records, nil
}

// GetDomainRecord 查询单条解析记录详情
func (c *HuaweiCloudClient) GetDomainRecord(ctx context.Context, fulldomain, recordID string) (*DNSRecord, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(ctx, token, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/v2/zones/%s/recordsets/%s", c.DNSURL, zoneID, recordID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get record, status: %d", resp.StatusCode)
	}

	var record DNSRecord
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		return nil, err
	}

	return &record, nil
}

// getToken retrieves an authentication token from Huawei Cloud IAM
func (c *HuaweiCloudClient) getToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	if time.Now().Before(c.tokenExpiry) && c.cachedToken != "" {
		token := c.cachedToken
		expiry := c.tokenExpiry
		c.tokenMu.Unlock()
		slog.Debug("使用缓存的华为云 IAM token",
			"expires_at", expiry.Format(time.RFC3339))
		return token, nil
	}
	c.tokenMu.Unlock()

	slog.Info("获取华为云 IAM token")

	authRequest := map[string]any{
		"auth": map[string]any{
			"identity": map[string]any{
				"methods": []string{"password"},
				"password": map[string]any{
					"user": map[string]any{
						"name":     c.Username,
						"password": c.Password,
						"domain": map[string]any{
							"name": c.DomainName,
						},
					},
				},
			},
			"scope": map[string]any{
				"project": map[string]any{
					"name": "ap-southeast-1",
				},
			},
		},
	}

	body, err := json.Marshal(authRequest)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.IAMURL+"/v3/auth/tokens", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("获取华为云 IAM token 失败",
			"status", resp.StatusCode)
		return "", fmt.Errorf("failed to get token, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	token := resp.Header.Get("X-Subject-Token")
	if token == "" {
		return "", fmt.Errorf("token not found in response")
	}

	// 双重检查锁：避免并发获取时重复请求 IAM
	c.tokenMu.Lock()
	c.cachedToken = token
	c.tokenExpiry = time.Now().Add(20 * time.Hour)
	c.tokenMu.Unlock()

	slog.Info("华为云 IAM token 获取成功并已缓存",
		"expires_in", "20h")
	return token, nil
}

// getZoneID finds the zone ID for a given domain
func (c *HuaweiCloudClient) getZoneID(ctx context.Context, token, domain string) (string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")
		slog.Debug("查找华为云 zone", "domain", h)

		req, err := http.NewRequestWithContext(ctx, "GET", c.DNSURL+"/v2/zones?name="+url.QueryEscape(h), nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("X-Auth-Token", token)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return "", err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		var result struct {
			Zones []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"zones"`
		}

		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil {
			continue
		}

		for _, zone := range result.Zones {
			if zone.Name == h+"." {
				slog.Info("华为云 zone 已找到",
					"zone", h, "zone_id", zone.ID)
				return zone.ID, nil
			}
		}
	}

	// 兜底：完整域名即为区域
	req, err := http.NewRequestWithContext(ctx, "GET", c.DNSURL+"/v2/zones?name="+url.QueryEscape(domain), nil)
	if err != nil {
		return "", fmt.Errorf("zone not found for domain %s", domain)
	}
	req.Header.Set("X-Auth-Token", token)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query zone for domain %s: %w", domain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var result struct {
			Zones []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"zones"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			for _, zone := range result.Zones {
				if zone.Name == domain+"." {
					slog.Info("华为云 zone 已找到（兜底）",
						"zone", domain, "zone_id", zone.ID)
					return zone.ID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("zone not found for domain %s", domain)
}

// createRecordSet creates a new DNS record set
func (c *HuaweiCloudClient) createRecordSet(ctx context.Context, token, zoneID string, record DNSRecord) (DNSRecord, error) {
	slog.Info("创建华为云 DNS 记录集",
		"type", record.Type, "name", record.Name, "zone_id", zoneID)

	body, err := json.Marshal(record)
	if err != nil {
		return DNSRecord{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/v2/zones/%s/recordsets", c.DNSURL, zoneID), bytes.NewBuffer(body))
	if err != nil {
		return DNSRecord{}, err
	}
	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		slog.Error("创建华为云 DNS 记录集失败",
			"type", record.Type, "name", record.Name, "err", err)
		return DNSRecord{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("创建华为云 DNS 记录集返回错误状态码",
			"status", resp.StatusCode, "type", record.Type)
		return DNSRecord{}, fmt.Errorf("failed to create record set, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdRecord DNSRecord
	if err := json.NewDecoder(resp.Body).Decode(&createdRecord); err != nil {
		return DNSRecord{}, err
	}

	return createdRecord, nil
}

// updateRecordSet updates an existing DNS record set
func (c *HuaweiCloudClient) updateRecordSet(ctx context.Context, token, zoneID, recordID string, record DNSRecord) (DNSRecord, error) {
	slog.Info("更新华为云 DNS 记录集",
		"record_id", recordID, "type", record.Type, "zone_id", zoneID)

	body, err := json.Marshal(record)
	if err != nil {
		return DNSRecord{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", fmt.Sprintf("%s/v2/zones/%s/recordsets/%s", c.DNSURL, zoneID, recordID), bytes.NewBuffer(body))
	if err != nil {
		return DNSRecord{}, err
	}
	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		slog.Error("更新华为云 DNS 记录集失败",
			"record_id", recordID, "err", err)
		return DNSRecord{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("更新华为云 DNS 记录集返回错误状态码",
			"status", resp.StatusCode, "record_id", recordID)
		return DNSRecord{}, fmt.Errorf("failed to update record set, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var updatedRecord DNSRecord
	if err := json.NewDecoder(resp.Body).Decode(&updatedRecord); err != nil {
		return DNSRecord{}, err
	}

	return updatedRecord, nil
}

// deleteRecordSet deletes a DNS record set
func (c *HuaweiCloudClient) deleteRecordSet(ctx context.Context, token, zoneID, recordID string) error {
	slog.Info("删除华为云 DNS 记录集",
		"record_id", recordID, "zone_id", zoneID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/v2/zones/%s/recordsets/%s", c.DNSURL, zoneID, recordID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		slog.Error("删除华为云 DNS 记录集失败",
			"record_id", recordID, "err", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("删除华为云 DNS 记录集返回错误状态码",
			"status", resp.StatusCode, "record_id", recordID)
		return fmt.Errorf("failed to delete record set, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
