package alicloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

var ctx = context.Background()

func TestAddRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1, "RecordId": "123456"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.AddRecord(ctx, ddns.RecordInfo{Name: "test.example.com", Type: "A", Value: "192.168.1.1", TTL: 600})
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

func TestModifyRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1, "RecordId": "123456"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.ModifyRecord(ctx, ddns.RecordInfo{Name: "test.example.com", ID: "123456", Type: "A", Value: "192.168.1.2", TTL: 600})
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

func TestDeleteRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.DeleteRecord(ctx, ddns.RecordInfo{Name: "test.example.com", ID: "123456"})
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

func TestGetRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1, "DomainRecords": {"Record": [{"RecordId": "123456", "Domain": "example.com", "RR": "test", "Type": "A", "Value": "192.168.1.1", "TTL": 600}]}}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	records, err := client.GetRecords(ctx, "test.example.com", "A")
	if err != nil {
		t.Errorf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

func TestGetDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1, "RecordId": "123456", "Domain": "example.com", "RR": "test", "Type": "A", "Value": "192.168.1.1", "TTL": 600}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	record, err := client.GetDomainRecord(ctx, "test.example.com", "123456")
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

	_, _, err := client.getRootDomain(ctx, "test.example.com")
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

	_, err := client.makeV1Request(ctx, map[string]string{"Action": "TestAction"})
	if err != nil {
		t.Errorf("makeV1Request failed: %v", err)
	}
}
