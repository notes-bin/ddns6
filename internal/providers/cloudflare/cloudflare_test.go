package cloudflare

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var ctx = context.Background()

// newCloudflareTestServer creates a test server that handles both zone lookup and record operations
func newCloudflareTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/zones") && !strings.Contains(r.URL.Path, "/dns_records") {
			// Zone lookup
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": [{"id": "zone123", "name": "example.com"}]}`))
		} else if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "GET" && !strings.Contains(r.URL.Path, "/dns_records/") {
			// List records - array response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": [], "result_info": {"page": 1, "per_page": 100, "total_pages": 1, "total_count": 0}}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": {"id": "123456", "type": "A", "name": "test.example.com", "content": "192.168.1.1", "ttl": 600}}`))
		}
	}))
}

func TestAddRecord(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	err := client.AddRecord(ctx, "test.example.com", "A", "192.168.1.1", 600)
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

func TestModifyRecord(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	err := client.ModifyRecord(ctx, "test.example.com", "123456", "A", "192.168.1.2", 600)
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

func TestDeleteRecord(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	err := client.DeleteRecord(ctx, "test.example.com", "123456")
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

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

	records, err := client.GetRecords(ctx, "test.example.com", "A")
	if err != nil {
		t.Errorf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

func TestGetDomainRecord(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	record, err := client.GetDomainRecord(ctx, "test.example.com", "123456")
	if err != nil {
		t.Fatalf("GetDomainRecord failed: %v", err)
	}

	if record.ID != "123456" {
		t.Errorf("Expected record ID 123456, got %s", record.ID)
	}
}

func TestGetZoneID(t *testing.T) {
	ts := newCloudflareTestServer(t)
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	zoneID, err := client.getZoneID(ctx, "test.example.com")
	if err != nil {
		t.Errorf("getZoneID failed: %v", err)
	}

	if zoneID != "zone123" {
		t.Errorf("Expected zone ID 'zone123', got '%s'", zoneID)
	}
}

func TestMakeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true, "result": {"status": "ok"}}`))
	}))
	defer ts.Close()

	client := NewClient(WithAPIToken("test-token"), WithBaseURL(ts.URL))

	var result map[string]interface{}
	err := client.makeRequest(ctx, "GET", ts.URL, nil, &result)
	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", result["status"])
	}
}
