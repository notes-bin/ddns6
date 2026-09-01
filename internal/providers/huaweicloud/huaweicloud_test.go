package huaweicloud

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// newHuaweiTestServer 创建 mock 服务器，校验 SDK-HMAC-SHA256 签名头。
func newHuaweiTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if r.Header.Get("X-Sdk-Date") == "" {
			t.Error("missing X-Sdk-Date header")
		}

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/v2/zones") && !strings.Contains(r.URL.Path, "/recordsets") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"zones": [{"id": "zone123", "name": "example.com."}]}`))
			return
		}
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"recordsets": [{"id": "123456", "name": "test.example.com.", "type": "AAAA", "records": ["2001:db8::1"], "ttl": 300}]}`))
		} else {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"id": "123456", "name": "test.example.com.", "type": "AAAA", "records": ["2001:db8::1"], "ttl": 300}`))
		}
	}))
}

// TestAddRecord 验证创建 recordset 成功路径。
func TestAddRecord(t *testing.T) {
	ts := newHuaweiTestServer(t)
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 300})
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

// TestModifyRecord 验证更新 recordset 成功路径。
func TestModifyRecord(t *testing.T) {
	ts := newHuaweiTestServer(t)
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "123456", Type: "AAAA", Value: "2001:db8::2", TTL: 300})
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

// TestDeleteRecord 验证删除 recordset 成功路径。
func TestDeleteRecord(t *testing.T) {
	ts := newHuaweiTestServer(t)
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "123456"})
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

// TestGetRecords 验证分页查询 recordsets 解析。
func TestGetRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/v2/zones") && !strings.Contains(r.URL.Path, "/recordsets") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"zones": [{"id": "zone123", "name": "example.com."}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"recordsets": [{"id": "123456", "name": "test.example.com.", "type": "AAAA", "records": ["2001:db8::1"], "ttl": 300}]}`))
	}))
	defer ts.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(ts.URL))

	records, err := client.GetRecords(t.Context(), "test.example.com", "AAAA")
	if err != nil {
		t.Errorf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
	if records[0].Value != "2001:db8::1" {
		t.Errorf("Expected value '2001:db8::1', got '%s'", records[0].Value)
	}
}
