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

type cloudflareStatus struct {
	Errors struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Messages struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"messages"`
	Success bool `json:"success"`
}

func (ss *cloudflareStatus) Error() string {
	return fmt.Sprintf("code: %d, message: %s", ss.Errors.Code, ss.Errors.Message)
}

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

func (res *Result) String() string {
	return fmt.Sprintf("id: %s, name: %s, type: %s, content: %s", res.Id, res.Name, res.Type, res.Content)
}

type cloudflareResponse struct {
	cloudflareStatus
	Result     []Result `json:"result"`
	ResultInfo struct {
		Count       int `json:"count"`
		Page        int `json:"page"`
		Per_page    int `json:"per_page"`
		Total_count int `json:"total_count"`
	} `json:"result_info"`
}

type cloudflareZoneResponse struct {
	cloudflareStatus
	Result []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
}

type cloudflareRequest struct {
	Comment   string `json:"comment,omitempty"`
	Content   string `json:"content,omitempty"`
	Name      string `json:"name,omitempty"`
	Ttl       int    `json:"ttl,omitempty"`
	Type      string `json:"type,omitempty"`
	Id        string `json:"id,omitempty"`
	Proxiable bool   `json:"proxiable,omitempty"`
}

type cloudflare struct {
	Token string
	*http.Client
}

var ErrIPv6NotChanged = errors.New("ipv6 address not changed")

func NewCloudflare(token string) *cloudflare {
	return &cloudflare{
		Token:  token,
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *cloudflare) Task(domain, subdomain, ipv6addr string) error {
	zones := new(cloudflareZoneResponse)
	response := new(cloudflareResponse)
	if err := c.getZones(domain, zones); err != nil {
		return err
	}
	zoneId := zones.Result[0].ID
	if err := c.ListRecords(domain, zoneId, response); err != nil {
		return err
	}
	if response.ResultInfo.Count == 0 {
		return c.CreateRecord(domain, subdomain, ipv6addr, zoneId, response)
	}
	for _, record := range response.Result {
		if record.Name == subdomain {
			if record.Content == ipv6addr {
				slog.Info("IPv6 地址未改变, 无法配置ddns", "domain", domain, "subdomain", subdomain, "ipv6", ipv6addr)
				return ErrIPv6NotChanged
			}
			if err := c.ModfiyRecord(domain, subdomain, zoneId, record.Id, ipv6addr, response); err != nil {
				return err
			}
			slog.Info("IPv6 地址发生变化, ddns配置完成", "domain", domain, "subdomain", subdomain, "ipv6", ipv6addr)
			return nil
		}
	}
	return nil
}

func (c *cloudflare) ListRecords(domain, zoneId string, response *cloudflareResponse) error {
	opts := cloudflareRequest{
		Name: domain,
	}
	if err := c.request("GET", fmt.Sprintf("%s/%s/%s", endpoint, zoneId, "dns_records"), &opts, &response); err != nil {
		return err
	}
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

func (c *cloudflare) CreateRecord(domain, subDomain, value, zoneId string, response *cloudflareResponse) error {
	opts := cloudflareRequest{
		Comment:   "",
		Content:   value,
		Name:      domain,
		Ttl:       3600,
		Type:      "AAAA",
		Proxiable: false,
	}
	if err := c.request("POST", fmt.Sprintf("%s/%s/dns_records", endpoint, zoneId), &opts, &response); err != nil {
		return err
	}
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

func (c *cloudflare) ModfiyRecord(domain, subDomain, zone_id, dns_record_id, value string, response *cloudflareResponse) error {
	opts := &cloudflareRequest{
		Name:      domain,
		Content:   value,
		Proxiable: true,
		Ttl:       3600,
		Type:      "AAAA",
	}
	if err := c.request("PATCH", fmt.Sprintf("%s/%s/dns_records/%s", endpoint, zone_id, dns_record_id), &opts, &response); err != nil {
		return err
	}
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

func (c *cloudflare) DeleteRecord(domain, zone_id, dns_record_id string, response *cloudflareResponse) error {
	opts := &cloudflareRequest{Name: domain}
	return c.request("DELETE", fmt.Sprintf("%s/%s/dns_records/%s", endpoint, zone_id, dns_record_id), &opts, &response)
}

func (c *cloudflare) getZones(domain string, response *cloudflareZoneResponse) error {
	params := url.Values{}
	params.Add("name", domain)
	params.Add("status", "active")
	params.Add("per_page", "50")

	if err := c.request("GET", fmt.Sprintf("%s/zones?%s", endpoint, params.Encode()), nil, &response); err != nil {
		return err
	}
	if !response.cloudflareStatus.Success {
		return &response.cloudflareStatus
	}
	return nil
}

func (c *cloudflare) validateToken() error {
	response := new(struct {
		Result struct {
			Id         string `json:"id"`
			Status     string `json:"status"`
			Not_before string `json:"not_before"`
			Expires_on string `json:"expires_on"`
		} `json:"result"`
		Success  bool     `json:"success"`
		Errors   []string `json:"errors"`
		Messages []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"messages"`
	})
	if err := c.request("GET", fmt.Sprintf("%s/%s", endpoint, "user/tokens/verify"), nil, response); err != nil {
		return err
	}
	slog.Debug("token is valid", "id", response.Result.Id, "status", response.Result.Status, "not_before", response.Result.Not_before, "expires_on", response.Result.Expires_on)
	if !response.Success {
		return errors.New("token is not valid")
	}
	return nil
}

func (c *cloudflare) request(method, apiUrl string, params, result any) (err error) {
	var jsonStr []byte
	if params != nil {
		jsonStr, err = json.Marshal(params)
		if err != nil {
			return
		}
	}

	req, err := http.NewRequest(method, apiUrl, bytes.NewBuffer(jsonStr))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	resp, err := c.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, error: %s", resp.StatusCode, resp.Status)
	}

	raw, err := io.ReadAll(resp.Body)
	slog.Debug("http response", "response", string(raw), "error", err)
	if err != nil {
		return
	}

	if err = json.Unmarshal(raw, &result); err != nil {
		slog.Error("json unmarshal error", "response", string(raw), "error", err)
		return
	}

	return nil
}
