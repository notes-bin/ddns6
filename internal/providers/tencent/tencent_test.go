package tencent_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/internal/providers/tencent"
)

// domainListResponse 用于 DescribeDomainList 的 mock 响应。
const domainListResponse = `{"Response": {"DomainList": [{"DomainId": 1, "Name": "example.com"}]}}`

// TestAddRecord 验证 CreateRecord 成功路径。
func TestAddRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") == "DescribeDomainList" {
			w.Write([]byte(domainListResponse))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response": {"RecordId": 123456}}`))
	}))
	defer ts.Close()

	client := tencent.NewDNSPod("testId", "testKey", tencent.WithBaseURL(ts.URL))

	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", Type: "A", Value: "192.168.1.1", TTL: 600})
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

// TestModifyRecord 验证 ModifyRecord 成功路径。
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

// TestDeleteRecord 验证 DeleteRecord 成功路径。
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

// TestGetRecords 验证 DescribeRecordList 解析与过滤。
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

// TestGetDomainRecord 验证 DescribeRecord 单条查询。
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

// TestAddRecord_AlreadyExists 验证记录已存在时同值跳过、异值改写。
func TestAddRecord_AlreadyExists(t *testing.T) {
	const existErr = `{"Response":{"Error":{"Code":"InvalidParameter.DomainRecordExist","Message":"record exists"}}}`
	tests := []struct {
		name         string
		listBody     string
		wantModified bool
	}{
		{
			name:     "同值跳过",
			listBody: `{"Response":{"RecordList":[{"RecordId":1,"Domain":"example.com","SubDomain":"@","RecordType":"AAAA","Value":"2001:db8::1","TTL":600}]}}`,
		},
		{
			name:         "异值更新",
			listBody:     `{"Response":{"RecordList":[{"RecordId":99,"Domain":"example.com","SubDomain":"@","RecordType":"AAAA","Value":"2001:db8::old","TTL":600}]}}`,
			wantModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var modified bool
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Header.Get("X-TC-Action") {
				case "DescribeDomainList":
					w.Write([]byte(domainListResponse))
				case "CreateRecord":
					w.Write([]byte(existErr))
				case "DescribeRecordList":
					w.Write([]byte(tt.listBody))
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
				t.Fatalf("AddRecord: %v", err)
			}
			if modified != tt.wantModified {
				t.Errorf("ModifyRecord called=%v, want %v", modified, tt.wantModified)
			}
		})
	}
}

// TestGetRootDomain_ProbeFallback 验证域名列表失败时回退探测逻辑。
func TestGetRootDomain_ProbeFallback(t *testing.T) {
	var listCalls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-TC-Action") {
		case "DescribeDomainList":
			w.Write([]byte(`{"Response":{"Error":{"Code":"AuthFailure","Message":"fail"}}}`))
		case "DescribeRecordList":
			listCalls++
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
	if listCalls == 0 {
		t.Fatal("expected DescribeRecordList probe calls after domain list failure")
	}
	if len(records) != 0 {
		t.Fatalf("expected empty records, got %+v", records)
	}
}

// TestApiError 验证非 2xx HTTP 状态被正确返回。
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
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected status 403 in error, got: %v", err)
	}
}
