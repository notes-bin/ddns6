package tencent_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"

	"github.com/notes-bin/ddns6/internal/providers/tencent"
)

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
		w.Write([]byte(`{"Response": {"RecordId": 123456}}`))
	}))
	defer ts.Close()

	// 创建客户端
	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))

	// 测试添加记录
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", Type: "A", Value: "192.168.1.1", TTL: 600})
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

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))

	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "123456", Type: "A", Value: "192.168.1.2", TTL: 600})
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

	client := tencent.NewDNSPod("testId", "Key", tencent.WithBaseURL(ts.URL))

	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "123456"})
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
		w.Write([]byte(`{"Response": {"Domain": "example.com", "RecordList": [{"RecordId": 123456, "Domain": "example.com", "SubDomain": "test", "RecordType": "A", "Value": "192.168.1.1", "TTL": 600}]}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))

	records, err := client.GetRecords(t.Context(), "test.example.com", "A")
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
		w.Write([]byte(`{"Response": {"RecordInfo": {"RecordId": 123456, "Domain": "example.com", "SubDomain": "test", "RecordType": "A", "Value": "192.168.1.1", "TTL": 600}}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))

	record, err := client.GetDomainRecord(t.Context(), "test.example.com", "123456")
	if err != nil {
		t.Fatalf("GetDomainRecord failed: %v", err)
	}

	if record.RecordId != 123456 {
		t.Errorf("Expected record ID 123456, got %d", record.RecordId)
	}
}

func TestAddRecord_AlreadyExistsSameValue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-TC-Action") {
		case "DescribeDomainList":
			w.Write([]byte(domainListResponse))
		case "CreateRecord":
			w.Write([]byte(`{"Response":{"Error":{"Code":"InvalidParameter.DomainRecordExist","Message":"record exists"}}}`))
		case "DescribeRecordList":
			w.Write([]byte(`{"Response":{"RecordList":[{"RecordId":1,"Domain":"example.com","SubDomain":"@","RecordType":"AAAA","Value":"2001:db8::1","TTL":600}]}}`))
		default:
			w.Write([]byte(`{"Response":{}}`))
		}
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{
		Name: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600,
	})
	if err != nil {
		t.Fatalf("AddRecord should skip same-value duplicate: %v", err)
	}
}

func TestAddRecord_AlreadyExistsUpdate(t *testing.T) {
	var modified bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-TC-Action") {
		case "DescribeDomainList":
			w.Write([]byte(domainListResponse))
		case "CreateRecord":
			w.Write([]byte(`{"Response":{"Error":{"Code":"InvalidParameter.DomainRecordExist","Message":"record exists"}}}`))
		case "DescribeRecordList":
			w.Write([]byte(`{"Response":{"RecordList":[{"RecordId":99,"Domain":"example.com","SubDomain":"@","RecordType":"AAAA","Value":"2001:db8::old","TTL":600}]}}`))
		case "ModifyRecord":
			modified = true
			w.Write([]byte(`{"Response":{"RequestId":"req-1"}}`))
		default:
			w.Write([]byte(`{"Response":{}}`))
		}
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{
		Name: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600,
	})
	if err != nil {
		t.Fatalf("AddRecord should update different-value duplicate: %v", err)
	}
	if !modified {
		t.Error("expected ModifyRecord to be called")
	}
}

func TestGetRootDomain_ProbeFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-TC-Action") {
		case "DescribeDomainList":
			w.Write([]byte(`{"Response":{"Error":{"Code":"AuthFailure","Message":"fail"}}}`))
		case "DescribeRecordList":
			w.Write([]byte(`{"Response":{"RecordList":[]}}`))
		default:
			w.Write([]byte(`{"Response":{}}`))
		}
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))
	records, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err != nil {
		t.Fatalf("probe fallback GetRecords failed: %v", err)
	}
	if records == nil {
		t.Fatal("expected non-nil records slice")
	}
}

func TestApiError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") == "DescribeDomainList" {
			w.Write([]byte(domainListResponse))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"Response":{"Error":{"Code":"AuthFailure","Message":"denied"}}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))
	_, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
