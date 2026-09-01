package alicloud

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// TestAddRecord 验证 AddDomainRecord 成功路径。
func TestAddRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1, "RecordId": "123456"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", Type: "A", Value: "192.168.1.1", TTL: 600})
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

// TestModifyRecord 验证 UpdateDomainRecord 成功路径。
func TestModifyRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1, "RecordId": "123456"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "123456", Type: "A", Value: "192.168.1.2", TTL: 600})
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

// TestDeleteRecord 验证 DeleteDomainRecord 成功路径。
func TestDeleteRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "123456"})
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

// TestGetRecords 验证 DescribeDomainRecords 解析。
func TestGetRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1, "DomainRecords": {"Record": [{"RecordId": "123456", "Domain": "example.com", "RR": "test", "Type": "A", "Value": "192.168.1.1", "TTL": 600}]}}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	records, err := client.GetRecords(t.Context(), "test.example.com", "A")
	if err != nil {
		t.Errorf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

// TestGetDomainRecord 验证 DescribeDomainRecordInfo 解析。
func TestGetDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1, "RecordId": "123456", "Domain": "example.com", "RR": "test", "Type": "A", "Value": "192.168.1.1", "TTL": 600}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	record, err := client.GetDomainRecord(t.Context(), "test.example.com", "123456")
	if err != nil {
		t.Errorf("GetDomainRecord failed: %v", err)
	}

	if record.RecordId != "123456" {
		t.Errorf("Expected record ID 123456, got %s", record.RecordId)
	}
}

// TestGetRootDomain 验证根域名逐级探测逻辑。
func TestGetRootDomain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "TotalCount": 1}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	_, _, err := client.getRootDomain(t.Context(), "test.example.com")
	if err != nil {
		t.Errorf("getRootDomain failed: %v", err)
	}
}

// TestMakeRequest 验证 V1 签名请求成功路径。
func TestMakeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id", "Status": "OK"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	_, err := client.makeV1Request(t.Context(), map[string]string{"Action": "TestAction"})
	if err != nil {
		t.Errorf("makeV1Request failed: %v", err)
	}
}

// TestMakeV3Request 验证 V3 签名头与 ACS3-HMAC-SHA256 格式。
func TestMakeV3Request(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if !strings.Contains(r.Header.Get("Authorization"), "ACS3-HMAC-SHA256") {
			t.Error("Authorization header should use ACS3-HMAC-SHA256")
		}
		if r.Header.Get("x-acs-action") != "TestAction" {
			t.Errorf("x-acs-action = %q, want TestAction", r.Header.Get("x-acs-action"))
		}
		if r.Header.Get("x-acs-version") != "2015-01-09" {
			t.Errorf("x-acs-version = %q, want 2015-01-09", r.Header.Get("x-acs-version"))
		}
		if r.Header.Get("x-acs-date") == "" {
			t.Error("missing x-acs-date header")
		}
		if r.Header.Get("x-acs-content-sha256") == "" {
			t.Error("missing x-acs-content-sha256 header")
		}
		if r.Header.Get("x-acs-signature-nonce") == "" {
			t.Error("missing x-acs-signature-nonce header")
		}
		if r.Host != ts.Listener.Addr().String() {
			t.Errorf("Host = %q, want %q", r.Host, ts.Listener.Addr().String())
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"RequestId": "test-request-id"}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	_, err := client.makeV3Request(t.Context(), map[string]string{"Action": "TestAction"})
	if err != nil {
		t.Errorf("makeV3Request failed: %v", err)
	}
}
