// Package namecheap 实现 Namecheap DNS API 服务。
//
// 对应 acme.sh dns_namecheap。
// 认证：API Key + Username + Client IP（Namecheap 要求）。
//
// API 文档：https://www.namecheap.com/support/api/intro.aspx
package namecheap

import (
	"cmp"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/domainutil"
)

const defaultBaseURL = "https://api.namecheap.com/xml.response"

// Client Namecheap DNS API 客户端。
type Client struct {
	apiKey     string
	username   string
	clientIP   string
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// NewClient 创建 Namecheap 客户端。
func NewClient(apiKey, username, clientIP string, options ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		username:   username,
		clientIP:   clientIP,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

// WithBaseURL 设置自定义 API 地址（测试用）。
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

type apiReply struct {
	Status string `xml:"Status,attr"`
	Errors struct {
		Error []struct {
			Text string `xml:",chardata"`
		} `xml:"Error"`
	} `xml:"Errors"`
	CommandResponse struct {
		DomainDNSGetHostsResult struct {
			Hosts []hostEntry `xml:"host"`
		} `xml:"DomainDNSGetHostsResult"`
	} `xml:"CommandResponse"`
}

type hostEntry struct {
	Name    string `xml:"Name,attr"`
	Type    string `xml:"Type,attr"`
	Address string `xml:"Address,attr"`
	MXPref  string `xml:"MXPref,attr"`
	TTL     string `xml:"TTL,attr"`
	HostID  string `xml:"HostId,attr"`
}

// AddRecord 添加 DNS 记录（通过 setHosts 重写主机列表）。
func (c *Client) AddRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.upsert(ctx, info, false)
}

// ModifyRecord 修改 DNS 记录。
func (c *Client) ModifyRecord(ctx context.Context, info ddns.RecordInfo) error {
	return c.upsert(ctx, info, true)
}

// DeleteRecord 删除 DNS 记录。
func (c *Client) DeleteRecord(ctx context.Context, info ddns.RecordInfo) error {
	sld, tld, sub, err := c.splitDomain(info.Name, info.Zone)
	if err != nil {
		return err
	}
	hosts, err := c.getHosts(ctx, sld, tld)
	if err != nil {
		return err
	}
	filtered := make([]hostEntry, 0, len(hosts))
	for _, h := range hosts {
		if h.Type == info.Type && strings.EqualFold(h.Name, sub) && h.Address == info.Value {
			continue
		}
		filtered = append(filtered, h)
	}
	return c.setHosts(ctx, sld, tld, filtered)
}

// GetRecords 查询 DNS 记录。
func (c *Client) GetRecords(ctx context.Context, fulldomain, recordType string) ([]ddns.RecordInfo, error) {
	sld, tld, sub, err := c.splitDomain(fulldomain, "")
	if err != nil {
		return nil, err
	}
	hosts, err := c.getHosts(ctx, sld, tld)
	if err != nil {
		return nil, err
	}
	zone := sld + "." + tld
	result := make([]ddns.RecordInfo, 0, len(hosts))
	for _, h := range hosts {
		if recordType != "" && h.Type != recordType {
			continue
		}
		if sub != "@" && !strings.EqualFold(h.Name, sub) {
			continue
		}
		ttl, _ := strconv.Atoi(h.TTL)
		name := zone
		if h.Name != "" && !strings.EqualFold(h.Name, "@") {
			name = h.Name + "." + zone
		}
		result = append(result, ddns.RecordInfo{
			ID:    cmp.Or(h.HostID, h.Name+"|"+h.Type+"|"+h.Address),
			Name:  name,
			Zone:  zone,
			Type:  h.Type,
			Value: h.Address,
			TTL:   ttl,
		})
	}
	return result, nil
}

func (c *Client) upsert(ctx context.Context, info ddns.RecordInfo, replace bool) error {
	sld, tld, sub, err := c.splitDomain(info.Name, info.Zone)
	if err != nil {
		return err
	}
	hosts, err := c.getHosts(ctx, sld, tld)
	if err != nil {
		return err
	}
	ttl := strconv.Itoa(cmp.Or(info.TTL, ddns.DefaultTTL))
	newHosts := make([]hostEntry, 0, len(hosts)+1)
	replaced := false
	for _, h := range hosts {
		if replace && h.Type == info.Type && strings.EqualFold(h.Name, sub) {
			if !replaced {
				newHosts = append(newHosts, hostEntry{
					Name: sub, Type: info.Type, Address: info.Value, MXPref: "10", TTL: ttl,
				})
				replaced = true
			}
			continue
		}
		newHosts = append(newHosts, h)
	}
	if !replaced {
		newHosts = append(newHosts, hostEntry{
			Name: sub, Type: info.Type, Address: info.Value, MXPref: "10", TTL: ttl,
		})
	}
	slog.Debug("updating Namecheap hosts", "module", "namecheap", "domain", sld+"."+tld, "type", info.Type)
	return c.setHosts(ctx, sld, tld, newHosts)
}

func (c *Client) getHosts(ctx context.Context, sld, tld string) ([]hostEntry, error) {
	reply, err := c.call(ctx, "namecheap.domains.dns.getHosts", url.Values{
		"SLD": {sld},
		"TLD": {tld},
	})
	if err != nil {
		return nil, err
	}
	return reply.CommandResponse.DomainDNSGetHostsResult.Hosts, nil
}

func (c *Client) setHosts(ctx context.Context, sld, tld string, hosts []hostEntry) error {
	params := url.Values{
		"SLD": {sld},
		"TLD": {tld},
	}
	for i := range len(hosts) {
		h := hosts[i]
		n := strconv.Itoa(i + 1)
		params.Set("HostName"+n, h.Name)
		params.Set("RecordType"+n, h.Type)
		params.Set("Address"+n, h.Address)
		params.Set("MXPref"+n, cmp.Or(h.MXPref, "10"))
		params.Set("TTL"+n, cmp.Or(h.TTL, strconv.Itoa(ddns.DefaultTTL)))
	}
	_, err := c.call(ctx, "namecheap.domains.dns.setHosts", params)
	return err
}

func (c *Client) call(ctx context.Context, command string, extra url.Values) (*apiReply, error) {
	params := url.Values{
		"ApiUser":  {c.username},
		"ApiKey":   {c.apiKey},
		"UserName": {c.username},
		"ClientIp": {c.clientIP},
		"Command":  {command},
	}
	for k, v := range extra {
		params[k] = v
	}
	endpoint := c.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Namecheap API request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Namecheap response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &httpStatusError{status: resp.StatusCode, body: string(body)}
	}
	var reply apiReply
	if err := xml.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("failed to decode Namecheap response: %w", err)
	}
	if reply.Status != "OK" {
		msg := "unknown error"
		if len(reply.Errors.Error) > 0 {
			msg = reply.Errors.Error[0].Text
		}
		return nil, fmt.Errorf("Namecheap API error: %s", msg)
	}
	return &reply, nil
}

func (c *Client) splitDomain(name, zoneHint string) (sld, tld, sub string, err error) {
	root, sub := domainutil.SplitDomain(name, zoneHint)
	parts := strings.Split(strings.ToLower(strings.TrimSuffix(root, ".")), ".")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid Namecheap domain: %s", name)
	}
	tld = parts[len(parts)-1]
	sld = parts[len(parts)-2]
	if sub == "" {
		sub = "@"
	}
	return sld, tld, sub, nil
}

// httpStatusError 表示 Namecheap API 返回的非 2xx 响应。
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("Namecheap API error: status %d, body: %s", e.status, e.body)
}
