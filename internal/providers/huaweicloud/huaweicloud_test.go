package huaweicloud

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"id": "123456"}`))
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithDNSURL(ts.URL))

	err := client.AddDomainRecord("test.example.com", "A", "192.168.1.1", 600)
	if err != nil {
		t.Errorf("AddDomainRecord failed: %v", err)
	}
}

func TestModifyDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"id": "123456"}`))
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithDNSURL(ts.URL))

	err := client.ModifyDomainRecord("test.example.com", "123456", "A", "192.168.1.2", 600)
	if err != nil {
		t.Errorf("ModifyDomainRecord failed: %v", err)
	}
}

func TestDeleteDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithDNSURL(ts.URL))

	err := client.DeleteDomainRecord("test.example.com", "123456")
	if err != nil {
		t.Errorf("DeleteDomainRecord failed: %v", err)
	}
}

func TestGetDomainRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"recordsets": [{"id": "123456", "name": "test.example.com.", "type": "A", "records": ["192.168.1.1"], "ttl": 600}]}`))
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithDNSURL(ts.URL))

	records, err := client.GetDomainRecords("test.example.com")
	if err != nil {
		t.Errorf("GetDomainRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

func TestGetDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "123456", "name": "test.example.com.", "type": "A", "records": ["192.168.1.1"], "ttl": 600}`))
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithDNSURL(ts.URL))

	record, err := client.GetDomainRecord("test.example.com", "123456")
	if err != nil {
		t.Errorf("GetDomainRecord failed: %v", err)
	}

	if record.ID != "123456" {
		t.Errorf("Expected record ID 123456, got %s", record.ID)
	}
}

func TestGetToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Subject-Token", "test-token")
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithIAMURL(ts.URL))

	token, err := client.getToken()
	if err != nil {
		t.Errorf("getToken failed: %v", err)
	}

	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", token)
	}
}

func TestGetZoneID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"zones": [{"id": "zone123", "name": "example.com."}]}`))
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithDNSURL(ts.URL))

	zoneID, err := client.getZoneID("test-token", "test.example.com")
	if err != nil {
		t.Errorf("getZoneID failed: %v", err)
	}

	if zoneID != "zone123" {
		t.Errorf("Expected zone ID 'zone123', got '%s'", zoneID)
	}
}
