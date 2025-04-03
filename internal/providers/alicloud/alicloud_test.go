package alicloud

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "RecordId": "123456"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.AddDomainRecord("test.example.com", "A", "192.168.1.1", 600)
	if err != nil {
		t.Errorf("AddDomainRecord failed: %v", err)
	}
}

func TestModifyDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "RecordId": "123456"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.ModifyDomainRecord("test.example.com", "123456", "A", "192.168.1.2", 600)
	if err != nil {
		t.Errorf("ModifyDomainRecord failed: %v", err)
	}
}

func TestDeleteDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.DeleteDomainRecord("test.example.com", "123456")
	if err != nil {
		t.Errorf("DeleteDomainRecord failed: %v", err)
	}
}

func TestGetDomainRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "DomainRecords": {"Record": [{"RecordId": "123456", "Domain": "example.com", "RR": "test", "Type": "A", "Value": "192.168.1.1", "TTL": 600}]}}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	records, err := client.GetDomainRecords("test.example.com", "A")
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
		w.Write([]byte(`{"RequestId": "test-request-id", "RecordId": "123456", "Domain": "example.com", "RR": "test", "Type": "A", "Value": "192.168.1.1", "TTL": 600}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	record, err := client.GetDomainRecord("test.example.com", "123456")
	if err != nil {
		t.Errorf("GetDomainRecord failed: %v", err)
	}

	if record.RecordId != "123456" {
		t.Errorf("Expected record ID 123456, got %s", record.RecordId)
	}
}

func TestGetRootDomain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	_, _, err := client.getRootDomain("test.example.com")
	if err != nil {
		t.Errorf("getRootDomain failed: %v", err)
	}
}

func TestMakeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "Status": "OK"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	_, err := client.makeRequest(map[string]string{"Action": "TestAction"})
	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}
}
