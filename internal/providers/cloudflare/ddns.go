// 包 cloudflare 提供了与 Cloudflare API 交互的功能，用于实现动态 DNS 更新。
// 它包含了处理 DNS 记录更新、删除以及查询等操作的函数和类型。
package cloudflare

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const endpoint = "https://api.cloudflare.com/client/v4"

// cloudflareStatus 表示 Cloudflare API 响应的状态信息
type cloudflareStatus struct {
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Messages []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"messages"`
	Success bool `json:"success"`
}

// Error 实现 error 接口，用于返回错误信息
func (ss *cloudflareStatus) Error() string {
	if ss.Success {
		return ""
	}
	return fmt.Sprintf("code: %d, message: %s", ss.Errors[0].Code, ss.Errors[0].Message)
}

// String 实现 Stringer 接口，用于返回消息信息
func (ss *cloudflareStatus) String() string {
	return fmt.Sprintf("code: %d, message: %s", ss.Messages[0].Code, ss.Messages[0].Message)
}

// Result 表示 DNS 记录的结果信息
type Result struct {
	Comment   string `json:"comment"`
	Content   string `json:"content"`
	Name      string `json:"name"`
	Ttl       int    `json:"ttl"`
	Type      string `json:"type"`
	Id        string `json:"id"`
	Proxiable bool   `json:"proxiable"`
	Proxied   bool   `json:"proxied"`
	Settings  struct {
		Ipv4_only bool `json:"ipv4_only"`
		Ipv6_only bool `json:"ipv6_only"`
	} `json:"settings"`
	Tags []string `json:"tags"`
}

// String 实现 Stringer 接口，用于返回结果信息
func (res *Result) String() string {
	return fmt.Sprintf("id: %s, name: %s, type: %s, content: %s", res.Id, res.Name, res.Type, res.Content)
}

// cloudflareResultInfo 表示 Cloudflare API 响应的结果信息
type cloudflareResultInfo struct {
	Count       int `json:"count"`
	Page        int `json:"page"`
	Per_page    int `json:"per_page"`
	Total_count int `json:"total_count"`
}

// cloudflareResponse 表示 Cloudflare API 响应的通用结构
type cloudflareResponse struct {
	cloudflareStatus
	Result      []Result             `json:"result"`
	Result_info cloudflareResultInfo `json:"result_info"`
}

// cloudflareZoneResponse 表示 Cloudflare API 响应的区域信息
type cloudflareZoneResponse struct {
	Result []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"result"`
	Result_info cloudflareResultInfo `json:"result_info"`
	cloudflareStatus
}

// cloudflareRequest 表示 Cloudflare API 请求的通用结构
type cloudflareRequest struct {
	Comment   string `json:"comment,omitempty"`
	Content   string `json:"content,omitempty"`
	Name      string `json:"name,omitempty"`
	Ttl       int    `json:"ttl,omitempty"`
	Type      string `json:"type,omitempty"`
	Id        string `json:"id,omitempty"`
	Proxiable bool   `json:"proxiable,omitempty"`
}

// cloudflare 表示 Cloudflare API 客户端
type cloudflare struct {
	Token string `env:"CF_Token" required:"true"`
	*http.Client
}

// ErrIPv6NotChanged 表示 IPv6 地址未改变的错误
var ErrIPv6NotChanged = errors.New("ipv6 address not changed")

// New 创建一个新的 Cloudflare API 客户端
func New() *cloudflare {
	return &cloudflare{Client: &http.Client{Timeout: 30 * time.Second}}
}

// Task 执行动态 DNS 更新任务
func (c *cloudflare) Task(domain, subdomain, ipv6addr string) error {
	if subdomain != "" {
		domain = fmt.Sprintf("%s.%s", subdomain, domain)
	}
	zones := new(cloudflareZoneResponse)
	response := new(cloudflareResponse)

	// 获取区域信息
	if err := c.getZones(domain, zones); err != nil {
		return fmt.Errorf("failed to get zones: %w", err)
	}

	if len(zones.Result) == 0 {
		return errors.New("域名不存在")
	}
	zoneId := zones.Result[0].ID

	// 列出 DNS 记录
	if err := c.listRecords(domain, zoneId, response); err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

	if response.Result_info.Count == 0 {
		// 创建 DNS 记录
		if err := c.createRecord(domain, ipv6addr, zoneId, response); err != nil {
			return fmt.Errorf("failed to create record: %w", err)
		}
		slog.Info("IPv6 地址记录创建成功", "domain", domain, "subdomain", subdomain, "ipv6", ipv6addr)
		return nil
	}

	for _, record := range response.Result {
		if record.Name == domain {
			if record.Content == ipv6addr {
				slog.Info("IPv6 地址未改变, 无需配置 ddns", "domain", domain, "subdomain", subdomain, "ipv6", ipv6addr)
				return ErrIPv6NotChanged
			}
			// 修改 DNS 记录
			if err := c.modifyRecord(domain, zoneId, record.Id, ipv6addr, response); err != nil {
				return fmt.Errorf("failed to modify record: %w", err)
			}
			slog.Info("IPv6 地址发生变化, ddns 配置完成", "domain", domain, "subdomain", subdomain, "ipv6", ipv6addr)
			return nil
		}
	}

	return nil
}

// listRecords 列出指定区域的 DNS 记录
func (c *cloudflare) listRecords(domain, zoneId string, response *cloudflareResponse) error {
	opts := &cloudflareRequest{
		Name: domain,
	}
	if err := c.request("GET", fmt.Sprintf("%s/zones/%s/dns_records", endpoint, zoneId), opts, response); err != nil {
		return err
	}
	slog.Debug("get records", "response", *response)
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

// createRecord 创建新的 DNS 记录
func (c *cloudflare) createRecord(domain, value, zoneId string, response *cloudflareResponse) error {
	opts := &cloudflareRequest{
		Comment:   fmt.Sprintf("create domain %s by ddns6", domain),
		Content:   value,
		Name:      domain,
		Ttl:       1,
		Type:      "AAAA",
		Proxiable: false,
	}
	if err := c.request("POST", fmt.Sprintf("%s/zones/%s/dns_records", endpoint, zoneId), opts, response); err != nil {
		return err
	}
	slog.Debug("create record", "params", opts, "response", *response)
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

// modifyRecord 修改现有的 DNS 记录
func (c *cloudflare) modifyRecord(domain, zone_id, dns_record_id, value string, response *cloudflareResponse) error {
	opts := &cloudflareRequest{
		Name:      domain,
		Content:   value,
		Proxiable: true,
		Ttl:       3600,
		Type:      "AAAA",
	}
	if err := c.request("PUT", fmt.Sprintf("%s/zones/%s/dns_records/%s", endpoint, zone_id, dns_record_id), opts, response); err != nil {
		return err
	}
	slog.Debug("modify record", "response", *response)
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

// deleteRecord 删除指定的 DNS 记录
func (c *cloudflare) deleteRecord(domain, zone_id, dns_record_id string, response *cloudflareResponse) error {
	opts := &cloudflareRequest{Name: domain}
	if err := c.request("DELETE", fmt.Sprintf("%s/zones/%s/dns_records/%s", endpoint, zone_id, dns_record_id), opts, response); err != nil {
		return err
	}
	slog.Debug("delete record", "response", *response)
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

// getZones 获取指定域名的区域信息
func (c *cloudflare) getZones(domain string, response *cloudflareZoneResponse) error {
	params := url.Values{}
	params.Add("name", domain)
	params.Add("status", "active")
	params.Add("per_page", "50")

	if err := c.request("GET", fmt.Sprintf("%s/zones?%s", endpoint, params.Encode()), nil, response); err != nil {
		return err
	}
	slog.Debug("get zones", "params", params.Encode(), "response", *response)
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

// validateToken 验证 Cloudflare API 令牌的有效性
func (c *cloudflare) validateToken() error {
	response := new(struct {
		Result struct {
			Id         string `json:"id"`
			Status     string `json:"status"`
			Not_before string `json:"not_before"`
			Expires_on string `json:"expires_on"`
		} `json:"result"`
		cloudflareStatus
	})
	if err := c.request("GET", fmt.Sprintf("%s/user/tokens/verify", endpoint), nil, response); err != nil {
		return err
	}
	slog.Debug("token is valid", "id", response.Result.Id, "status", response.Result.Status)
	if !response.Success {
		return errors.New("token is not valid")
	}
	slog.Info(response.cloudflareStatus.String(), "status", response.Result.Status, "expires_on", response.Result.Expires_on)
	return nil
}

// request 发送 HTTP 请求到 Cloudflare API
func (c *cloudflare) request(method, apiUrl string, params, result any) (err error) {
	var jsonStr []byte
	if params != nil {
		jsonStr, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal request params: %w", err)
		}
	}

	req, err := http.NewRequest(method, apiUrl, bytes.NewBuffer(jsonStr))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, error: %s", resp.StatusCode, resp.Status)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	slog.Debug("cloudflare http response", "response", string(raw), "error", err)

	if err = json.Unmarshal(raw, &result); err != nil {
		slog.Error("cloudflare json data unmarshal error", "response", string(raw), "error", err)
		return fmt.Errorf("failed to unmarshal response data: %w", err)
	}
	slog.Debug("Unmarshal cloudflare response", "response", result)
	return nil
}
