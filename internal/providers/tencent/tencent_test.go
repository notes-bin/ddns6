package tencent_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/notes-bin/ddns6/internal/providers/tencent"
)

func TestAddDomainRecord(t *testing.T) {
	// 创建测试服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RecordId": "123456"}}`))
	}))
	defer ts.Close()

	// 创建客户端
	client := tencent.NewDNSService("testId", "testKey", tencent.WithAPIUrl(ts.URL))

	// 测试添加记录
	err := client.AddDomainRecord("test.example.com", "A", "192.168.1.1", 600)
	if err != nil {
		t.Errorf("AddDomainRecord failed: %v", err)
	}
}

func TestModifyDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RequestId": "req-123"}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSService("testId", "testKey", tencent.WithAPIUrl(ts.URL))

	err := client.ModifyDomainRecord("test.example.com", "123456", "A", "192.168.1.2", 600)
	if err != nil {
		t.Errorf("ModifyDomainRecord failed: %v", err)
	}
}

func TestDeleteDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RequestId": "req-123"}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSService("testId", "Key", tencent.WithAPIUrl(ts.URL))

	err := client.DeleteDomainRecord("test.example.com", "123456")
	if err != nil {
		t.Errorf("DeleteDomainRecord failed: %v", err)
	}
}

func TestGetDomainRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RecordList": [{"RecordId": "123456", "Domain": "example.com", "SubDomain": "test", "RecordType": "A", "Value": "192.168.1.1", "TTL": 600}]}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSService("testId", "testKey", tencent.WithAPIUrl(ts.URL))

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
		w.Write([]byte(`{"Response": {"RecordInfo": {"RecordId": "123456", "Domain": "example.com", "SubDomain": "test", "RecordType": "A", "Value": "192.168.1.1", "TTL": 600}}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSService("testId", "testKey", tencent.WithAPIUrl(ts.URL))

	record, err := client.GetDomainRecord("test.example.com", "123456")
	if err != nil {
		t.Errorf("GetDomainRecord failed: %v", err)
	}

	if record.RecordId != "123456" {
		t.Errorf("Expected record ID 123456, got %s", record.RecordId)
	}
}

func TestFindDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RecordList": [{"RecordId": "123456", "Domain": "example.com", "SubDomain": "test", "RecordType": "A", "Value": "192.168.1.1", "TTL": 600}]}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSService("testId", "testKey", tencent.WithAPIUrl(ts.URL))

	record, err := client.FindDomainRecord("test.example.com", "A", "192.168.1.1")
	if err != nil {
		t.Errorf("FindDomainRecord failed: %v", err)
	}

	if record == nil {
		t.Error("Expected a record, got nil")
	}
}
