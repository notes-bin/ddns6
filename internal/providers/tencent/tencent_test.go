package tencent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"

	"github.com/notes-bin/ddns6/internal/providers/tencent"
)

var ctx = context.Background()

// domainListResponse 用于 DescribeDomainList 的 mock 响应
const domainListResponse = `{"Response": {"DomainList": [{"DomainId": 1, "Name": "example.com"}]}}`

func TestAddRecord(t *testing.T) {
	// 创建测试服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") == "DescribeDomainList" {
			w.Write([]byte(domainListResponse))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RecordId": "123456"}}`))
	}))
	defer ts.Close()

	// 创建客户端
	client := tencent.NewDNSPod("testId", "testKey", tencent.WithAPIUrl(ts.URL))

	// 测试添加记录
	err := client.AddRecord(ctx, ddns.RecordInfo{Name: "test.example.com", Type: "A", Value: "192.168.1.1", TTL: 600})
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

func TestModifyRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") == "DescribeDomainList" {
			w.Write([]byte(domainListResponse))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RequestId": "req-123"}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithAPIUrl(ts.URL))

	err := client.ModifyRecord(ctx, ddns.RecordInfo{Name: "test.example.com", ID: "123456", Type: "A", Value: "192.168.1.2", TTL: 600})
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

func TestDeleteRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") == "DescribeDomainList" {
			w.Write([]byte(domainListResponse))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RequestId": "req-123"}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "Key", tencent.WithAPIUrl(ts.URL))

	err := client.DeleteRecord(ctx, ddns.RecordInfo{Name: "test.example.com", ID: "123456"})
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

func TestGetRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") == "DescribeDomainList" {
			w.Write([]byte(domainListResponse))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"Domain": "example.com", "RecordList": [{"RecordId": "123456", "Domain": "example.com", "SubDomain": "test", "RecordType": "A", "Value": "192.168.1.1", "TTL": 600}]}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithAPIUrl(ts.URL))

	records, err := client.GetRecords(ctx, "test.example.com", "A")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}
}

func TestGetDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") == "DescribeDomainList" {
			w.Write([]byte(domainListResponse))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RecordInfo": {"RecordId": "123456", "Domain": "example.com", "SubDomain": "test", "RecordType": "A", "Value": "192.168.1.1", "TTL": 600}}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithAPIUrl(ts.URL))

	record, err := client.GetDomainRecord(ctx, "test.example.com", "123456")
	if err != nil {
		t.Fatalf("GetDomainRecord failed: %v", err)
	}

	if record.RecordId != "123456" {
		t.Errorf("Expected record ID 123456, got %s", record.RecordId)
	}
}
