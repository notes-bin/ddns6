package godaddy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// newGoDaddyTestServer 创建同时处理域名查询与记录操作的 mock 服务器。
func newGoDaddyTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/domains/") && !strings.Contains(r.URL.Path, "/records/") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"domain": "example.com"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `[{"data": "192.168.1.1", "name": "test", "type": "A", "ttl": 600}]`)
		}
	}))
}

// TestAddRecord 验证 AddRecord 成功路径。
func TestAddRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", Type: "A", Value: "192.168.1.1", TTL: 600})
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

// TestModifyRecord 验证 ModifyRecord 按旧值匹配更新。
func TestModifyRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "192.168.1.1", Type: "A", Value: "192.168.1.2", TTL: 600})
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

// TestDeleteRecord 验证 DeleteRecord 按值删除。
func TestDeleteRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "192.168.1.1"})
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

// TestGetRecords 验证 GetRecords 列表解析。
func TestGetRecords(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	records, err := client.GetRecords(t.Context(), "test.example.com", "A")
	if err != nil {
		t.Errorf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

// TestGetRootDomain 验证 getRootDomain 根域解析。
func TestGetRootDomain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"domain": "example.com"}`))
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	subDomain, domain, err := client.getRootDomain(t.Context(), "test.example.com")
	if err != nil {
		t.Errorf("getRootDomain failed: %v", err)
	}

	if subDomain != "test" || domain != "example.com" {
		t.Errorf("Expected subDomain=test, domain=example.com, got subDomain=%s, domain=%s", subDomain, domain)
	}
}

// TestMakeRequest 验证 makeRequest 响应解码。
func TestMakeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	var result map[string]any
	err := client.makeRequest(t.Context(), "GET", ts.URL, nil, &result)
	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
}
