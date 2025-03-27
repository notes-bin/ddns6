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

// NewClient creates a new HuaweiCloudClient
func NewClient(username, password, domainName string, options ...func(*HuaweiCloudClient)) *HuaweiCloudClient {
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
func WithIAMURL(url string) func(*HuaweiCloudClient) {
	return func(c *HuaweiCloudClient) {
		c.IAMURL = url
	}
}

// WithDNSURL sets a custom DNS URL
func WithDNSURL(url string) func(*HuaweiCloudClient) {
	return func(c *HuaweiCloudClient) {
		c.DNSURL = url
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) func(*HuaweiCloudClient) {
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

// AddTxtRecord adds a TXT record to Huawei Cloud DNS
func (c *HuaweiCloudClient) AddTxtRecord(fulldomain, txtvalue string) error {
	token, err := c.getToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(token, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	// Get existing records
	existingRecords, err := c.getRecordSet(token, zoneID, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %v", err)
	}

	// Prepare new record value
	newRecord := fmt.Sprintf("\"%s\"", txtvalue)

	// Check if record already exists
	for _, record := range existingRecords.Records {
		if record == newRecord {
			// Record already exists
			return nil
		}
	}

	// Add new record to existing records
	updatedRecords := append(existingRecords.Records, newRecord)

	// Prepare updated record set
	updatedRecordSet := DNSRecord{
		Name:    fulldomain + ".",
		Type:    "TXT",
		TTL:     1,
		Records: updatedRecords,
	}

	// Update or create record set
	if existingRecords.ID != "" {
		_, err = c.updateRecordSet(token, zoneID, existingRecords.ID, updatedRecordSet)
	} else {
		updatedRecordSet.Description = "ACME Challenge"
		_, err = c.createRecordSet(token, zoneID, updatedRecordSet)
	}

	return err
}

// RemoveTxtRecord removes a TXT record from Huawei Cloud DNS
func (c *HuaweiCloudClient) RemoveTxtRecord(fulldomain, txtvalue string) error {
	token, err := c.getToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	zoneID, err := c.getZoneID(token, fulldomain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %v", err)
	}

	// Try multiple times to handle potential synchronization delays
	maxAttempts := 50
	for attempt := 0; attempt < maxAttempts; attempt++ {
		recordSet, err := c.getRecordSet(token, zoneID, fulldomain)
		if err != nil {
			return fmt.Errorf("failed to get record set: %v", err)
		}

		if recordSet.ID == "" {
			// Record set doesn't exist
			return nil
		}

		// Filter out the record to remove
		var updatedRecords []string
		targetRecord := fmt.Sprintf("\"%s\"", txtvalue)
		found := false
		for _, record := range recordSet.Records {
			if record != targetRecord {
				updatedRecords = append(updatedRecords, record)
			} else {
				found = true
			}
		}

		if !found {
			// Record not found, nothing to do
			return nil
		}

		if len(updatedRecords) == 0 {
			// No records left, delete the entire record set
			err = c.deleteRecordSet(token, zoneID, recordSet.ID)
			if err == nil {
				return nil
			}
		} else {
			// Update with remaining records
			updatedRecordSet := DNSRecord{
				Name:    fulldomain + ".",
				Type:    "TXT",
				TTL:     1,
				Records: updatedRecords,
			}
			_, err = c.updateRecordSet(token, zoneID, recordSet.ID, updatedRecordSet)
			if err == nil {
				return nil
			}
		}

		if attempt < maxAttempts-1 {
			// Wait before retrying
			continue
		}
	}

	return fmt.Errorf("failed to remove record after %d attempts", maxAttempts)
}

// getToken retrieves an authentication token from Huawei Cloud IAM
func (c *HuaweiCloudClient) getToken() (string, error) {
	authRequest := map[string]interface{}{
		"auth": map[string]interface{}{
			"identity": map[string]interface{}{
				"methods": []string{"password"},
				"password": map[string]interface{}{
					"user": map[string]interface{}{
						"name":     c.Username,
						"password": c.Password,
						"domain": map[string]interface{}{
							"name": c.DomainName,
						},
					},
				},
			},
			"scope": map[string]interface{}{
				"project": map[string]interface{}{
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

// getRecordSet retrieves a DNS record set
func (c *HuaweiCloudClient) getRecordSet(token, zoneID, name string) (DNSRecord, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/zones/%s/recordsets?name=%s&status=ACTIVE", c.DNSURL, zoneID, name), nil)
	if err != nil {
		return DNSRecord{}, err
	}
	req.Header.Set("X-Auth-Token", token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return DNSRecord{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DNSRecord{}, fmt.Errorf("failed to get record set, status: %d", resp.StatusCode)
	}

	var result struct {
		Recordsets []DNSRecord `json:"recordsets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return DNSRecord{}, err
	}

	if len(result.Recordsets) > 0 {
		return result.Recordsets[0], nil
	}

	return DNSRecord{}, nil
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
