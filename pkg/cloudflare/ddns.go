package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const endpoint = "https://api.cloudflare.com/client/v4/zones"

type Response struct {
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

type cloudflareResponse struct {
	cloudflareStatus
	Result     []Response
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
	Email string
	Key   string
	*http.Client
}

func NewCloudflare(email, key string) *cloudflare {
	return &cloudflare{
		Email:  email,
		Key:    key,
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *cloudflare) Task(domain, subdomain, ipv6addr string) error {
	zones := new(cloudflareZoneResponse)
	if err := c.getZoneId(domain, zones); err != nil {
		return err
	}
	zoneId := zones.Result[0].ID
	fmt.Println(zoneId)
	return nil
}

func (c *cloudflare) ListRecords(domain, zoneId string, response *cloudflareResponse) error {
	opts := cloudflareRequest{
		Name: domain,
	}
	return c.request("GET", fmt.Sprintf("%s/%s/%s", endpoint, zoneId, "dns_records"), &opts, &response)
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
	return c.request("POST", fmt.Sprintf("%s/zones/%s}/dns_records", endpoint, zoneId), &opts, &response)
}

func (c *cloudflare) ModfiyRecord(domain, subDomain, zone_id, dns_record_id, value string, response *cloudflareResponse) error {
	opts := &cloudflareRequest{
		Name:      domain,
		Content:   value,
		Proxiable: true,
		Ttl:       3600,
		Type:      "AAAA",
	}
	return c.request("PATCH", fmt.Sprintf("%s/zones/%s/dns_records/%s", endpoint, zone_id, dns_record_id), &opts, &response)
}

func (c *cloudflare) DeleteRecord(domain, zone_id, dns_record_id string, response *cloudflareResponse) error {
	opts := &cloudflareRequest{Name: domain}
	return c.request("DELETE", fmt.Sprintf("%s/zones/%s/dns_records/%s", endpoint, zone_id, dns_record_id), &opts, &response)
}

func (c *cloudflare) getZones(domain string, respnose *cloudflareZoneResponse) error {
	params := url.Values{}
	params.Add("name", domain)
	params.Add("status", "active")
	params.Add("per_page", "50")

	return c.request("GET", fmt.Sprintf("%s?%s", endpoint, params.Encode()), nil, &respnose)
}

func (c *cloudflare) request(method, apiUrl string, params, result any) error {
	jsonStr, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, apiUrl, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	slog.Debug("http response", "response", raw, "error", err)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}

	return nil
}
