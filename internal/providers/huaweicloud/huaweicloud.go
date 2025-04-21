package huaweicloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HuaweiCloudClient represents a client for Huawei Cloud DNS API
type HuaweiCloudClient struct {
	Username   string
	Password   string
	DomainName string
	IAMURL     string
	DNSURL     string
	HTTPClient *http.Client
}
type Options func(*HuaweiCloudClient)

// NewClient creates a new HuaweiCloudClient
func NewClient(username, password, domainName string, options ...Options) *HuaweiCloudClient {
	client := &HuaweiCloudClient{
		Username:   username,
		Password:   password,
		DomainName: domainName,
		IAMURL:     "https://iam.myhuaweicloud.com",
		DNSURL:     "https://dns.ap-southeast-1.myhuaweicloud.com",
		HTTPClient: &http.Client{},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithIAMURL sets a custom IAM URL
func WithIAMURL(url string) Options {
	return func(c *HuaweiCloudClient) {
		c.IAMURL = url
	}
}

// WithDNSURL sets a custom DNS URL
func WithDNSURL(url string) Options {
	return func(c *HuaweiCloudClient) {
		c.DNSURL = url
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) Options {
	return func(c *HuaweiCloudClient) {
		c.HTTPClient = httpClient
	}
}

// DNSRecord represents a Huawei Cloud DNS record
type DNSRecord struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	TTL         int      `json:"ttl,omitempty"`
	Records     []string `json:"records"`
}

// AddDomainRecord 添加域名解析记录
func (c *HuaweiCloudClient) AddDomainRecord(fulldomain, recordType, value string, ttl int) error {
	token, err := c.getToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(token, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	record := DNSRecord{
		Name:    fulldomain + ".",
		Type:    recordType,
		TTL:     ttl,
		Records: []string{value},
	}

	_, err = c.createRecordSet(token, zoneID, record)
	return err
}

// ModifyDomainRecord 修改域名解析记录
func (c *HuaweiCloudClient) ModifyDomainRecord(fulldomain, recordID, recordType, newValue string, ttl int) error {
	token, err := c.getToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(token, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	record := DNSRecord{
		Name:    fulldomain + ".",
		Type:    recordType,
		TTL:     ttl,
		Records: []string{newValue},
	}

	_, err = c.updateRecordSet(token, zoneID, recordID, record)
	return err
}

// DeleteDomainRecord 删除域名解析记录
func (c *HuaweiCloudClient) DeleteDomainRecord(fulldomain, recordID string) error {
	token, err := c.getToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(token, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	return c.deleteRecordSet(token, zoneID, recordID)
}

// GetDomainRecords 查询域名的所有解析记录
func (c *HuaweiCloudClient) GetDomainRecords(fulldomain string) ([]DNSRecord, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(token, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %v", err)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/zones/%s/recordsets", c.DNSURL, zoneID), nil)
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
		return nil, fmt.Errorf("failed to get records, status: %d", resp.StatusCode)
	}

	var result struct {
		Recordsets []DNSRecord `json:"recordsets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Recordsets, nil
}

// GetDomainRecord 查询特定解析记录
func (c *HuaweiCloudClient) GetDomainRecord(fulldomain, recordID string) (*DNSRecord, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(token, fulldomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone ID: %v", err)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/zones/%s/recordsets/%s", c.DNSURL, zoneID, recordID), nil)
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
func (c *HuaweiCloudClient) getToken() (string, error) {
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

	req, err := http.NewRequest("POST", c.IAMURL+"/v3/auth/tokens", bytes.NewBuffer(body))
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
		return "", fmt.Errorf("failed to get token, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	token := resp.Header.Get("X-Subject-Token")
	if token == "" {
		return "", fmt.Errorf("token not found in response")
	}

	return token, nil
}

// getZoneID finds the zone ID for a given domain
func (c *HuaweiCloudClient) getZoneID(token, domain string) (string, error) {
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		h := strings.Join(parts[i:], ".")

		req, err := http.NewRequest("GET", c.DNSURL+"/v2/zones?name="+h, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("X-Auth-Token", token)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to get zones, status: %d", resp.StatusCode)
		}

		var result struct {
			Zones []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"zones"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}

		for _, zone := range result.Zones {
			if zone.Name == h+"." {
				return zone.ID, nil
			}
		}
	}

	return "", fmt.Errorf("zone not found for domain %s", domain)
}

// createRecordSet creates a new DNS record set
func (c *HuaweiCloudClient) createRecordSet(token, zoneID string, record DNSRecord) (DNSRecord, error) {
	body, err := json.Marshal(record)
	if err != nil {
		return DNSRecord{}, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v2/zones/%s/recordsets", c.DNSURL, zoneID), bytes.NewBuffer(body))
	if err != nil {
		return DNSRecord{}, err
	}
	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return DNSRecord{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return DNSRecord{}, fmt.Errorf("failed to create record set, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdRecord DNSRecord
	if err := json.NewDecoder(resp.Body).Decode(&createdRecord); err != nil {
		return DNSRecord{}, err
	}

	return createdRecord, nil
}

// updateRecordSet updates an existing DNS record set
func (c *HuaweiCloudClient) updateRecordSet(token, zoneID, recordID string, record DNSRecord) (DNSRecord, error) {
	body, err := json.Marshal(record)
	if err != nil {
		return DNSRecord{}, err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/v2/zones/%s/recordsets/%s", c.DNSURL, zoneID, recordID), bytes.NewBuffer(body))
	if err != nil {
		return DNSRecord{}, err
	}
	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return DNSRecord{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return DNSRecord{}, fmt.Errorf("failed to update record set, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var updatedRecord DNSRecord
	if err := json.NewDecoder(resp.Body).Decode(&updatedRecord); err != nil {
		return DNSRecord{}, err
	}

	return updatedRecord, nil
}

// deleteRecordSet deletes a DNS record set
func (c *HuaweiCloudClient) deleteRecordSet(token, zoneID, recordID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v2/zones/%s/recordsets/%s", c.DNSURL, zoneID, recordID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete record set, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
