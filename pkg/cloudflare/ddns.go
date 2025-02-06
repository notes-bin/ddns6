package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	endpoint = "https://api.cloudflare.com/client/v4/zones"
	ID       = "CLOUDFLARE_AUTH_EMAIL"
	KEY      = "CLOUDFLARE_AUTH_KEY"
)

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
	Result []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
}

type cloudflareStatus struct {
	Success  bool `json:"success"`
	Messages struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"messages"`
}

type cloudflareRequest struct {
	Comment string `json:"comment,omitempty"`
	Content string `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`
	Ttl     int    `json:"ttl,omitempty"`
	Type    string `json:"type,omitempty"`
	Id      string `json:"id,omitempty"`
}

type cloudflare struct {
	Email string
	Key   string
}

func NewCloudflare(email, key string) *cloudflare {
	return &cloudflare{
		Email: email,
		Key:   key,
	}
}

func (c *cloudflare) Task(domain, subdomain, ipv6addr string) error {
	return nil
}

func (c *cloudflare) ListRecords(domain string, response *cloudflareResponse) error {
	return nil
}

func (c *cloudflare) CreateRecord(domain, subDomain, value string, status *cloudflareStatus) error {
	opts := cloudflareRequest{
		Comment: "",
		Content: value,
		Name:    subDomain,
		Ttl:     3600,
		Type:    "AAAA",
	}
	return c.request("POST", endpoint, &opts, &status)
}

func (c *cloudflare) ModfiyRecord(domain string, recordId int, subDomain, recordLine, value string, status *cloudflareStatus) error {
	return nil
}

func (c *cloudflare) DeleteRecord(domain string, RecordId int, status *cloudflareStatus) error {
	return nil
}

func (c *cloudflare) getZoneID(domain string, respnose *cloudflareZoneResponse) error {
	opts := cloudflareRequest{
		Name: domain,
	}
	return c.request("GET", endpoint, &opts, &respnose)
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

	cli := http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
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
