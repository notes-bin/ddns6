package cloudflare

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// newCloudflareTestServer 创建同时处理 Zone 查询与记录操作的 mock 服务器。
func newCloudflareTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/zones") && !strings.Contains(r.URL.Path, "/dns_records") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": [{"id": "zone123", "name": "example.com"}]}`))
		} else if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "GET" && !strings.Contains(r.URL.Path, "/dns_records/") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": [], "result_info": {"page": 1, "per_page": 100, "total_pages": 1, "total_count": 0}}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": {"id": "123456", "type": "A", "name": "test.example.com", "content": "192.168.1.1", "ttl": 600}}`))
		}
	}))
}

// TestAddRecord 验证 AddRecord 成功路径。
func TestAddRecord(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", Type: "A", Value: "192.168.1.1", TTL: 600})
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

// TestModifyRecord 验证 ModifyRecord 成功路径。
func TestModifyRecord(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "123456", Type: "A", Value: "192.168.1.2", TTL: 600})
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

// TestDeleteRecord 验证 DeleteRecord 成功路径。
func TestDeleteRecord(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "123456"})
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

// TestGetRecords 验证 GetRecords 列表解析。
func TestGetRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/zones") && !strings.Contains(r.URL.Path, "/dns_records") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": [{"id": "zone123", "name": "example.com"}]}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": [{"id": "123456", "type": "A", "name": "test.example.com", "content": "192.168.1.1", "ttl": 600}], "result_info": {"page": 1, "per_page": 100, "total_pages": 1, "total_count": 1}}`))
		}
	}))
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	records, err := client.GetRecords(t.Context(), "test.example.com", "A")
	if err != nil {
		t.Errorf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

// TestGetDomainRecord 验证 GetDomainRecord 单条查询。
func TestGetDomainRecord(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	record, err := client.GetDomainRecord(t.Context(), "test.example.com", "123456")
	if err != nil {
		t.Fatalf("GetDomainRecord failed: %v", err)
	}

	if record.ID != "123456" {
		t.Errorf("Expected record ID 123456, got %s", record.ID)
	}
}

// TestGetZoneID 验证 getZoneID 区域解析。
func TestGetZoneID(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	zoneID, err := client.getZoneID(t.Context(), "test.example.com")
	if err != nil {
		t.Errorf("getZoneID failed: %v", err)
	}

	if zoneID != "zone123" {
		t.Errorf("Expected zone ID 'zone123', got '%s'", zoneID)
	}
}

// TestMakeRequest 验证 makeRequest 响应解码。
func TestMakeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true, "result": {"status": "ok"}}`))
	}))
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	var result map[string]any
	err := client.makeRequest(t.Context(), "GET", ts.URL, nil, &result)
	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", result["status"])
	}
}

// TestApiError 验证 makeRequest 对 API 业务错误的透传。
func TestApiError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": false, "errors": [{"code": 9109, "message": "Invalid access token"}]}`))
	}))
	defer ts.Close()

	client := NewClient(WithAPIToken("bad-token"), WithBaseURL(ts.URL))
	var result map[string]any
	err := client.makeRequest(t.Context(), "GET", ts.URL+"/zones", nil, &result)
	if err == nil {
		t.Fatal("expected API error, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid access token") {
		t.Errorf("error should include API message: %v", err)
	}
}
