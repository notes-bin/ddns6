package dynv6

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// TestClient_GetRecords_Zone 验证根域名 zone 的 AAAA 记录查询。
func TestClient_GetRecords_Zone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/zones" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]Zone{
				{ID: "zone1", Name: "example.com", IPv6: "2001:db8::1"},
			})
			return
		}
		if r.URL.Path == "/api/v2/zones/zone1" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(Zone{ID: "zone1", Name: "example.com", IPv6: "2001:db8::1"})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	records, err := client.GetRecords(t.Context(), "example.com", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Value != "2001:db8::1" {
		t.Errorf("expected 2001:db8::1, got %s", records[0].Value)
	}
}

// TestClient_GetRecords_Subdomain 验证子域名通过 records 列表查询。
func TestClient_GetRecords_Subdomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/zones" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]Zone{
				{ID: "zone1", Name: "example.com"},
			})
			return
		}
		if r.URL.Path == "/api/v2/zones/zone1/records" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]Record{
				{ID: "rec1", Type: "AAAA", Name: "www", Data: "2001:db8::1"},
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	records, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Value != "2001:db8::1" {
		t.Errorf("expected 2001:db8::1, got %s", records[0].Value)
	}
}

// TestClient_AddRecord 验证为子域名创建 AAAA 记录。
func TestClient_AddRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/zones" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]Zone{
				{ID: "zone1", Name: "example.com"},
			})
			return
		}
		if r.URL.Path == "/api/v2/zones/zone1/records" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(Record{ID: "new-rec", Type: "AAAA", Name: "www", Data: "2001:db8::1"})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_ModifyRecord 验证 PATCH 更新已有记录。
func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/zones" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]Zone{
				{ID: "zone1", Name: "example.com"},
			})
			return
		}
		if r.URL.Path == "/api/v2/zones/zone1/records/rec1" && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Record{ID: "rec1", Type: "AAAA", Data: "2001:db8::2"})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "www.example.com", ID: "rec1", Type: "AAAA", Value: "2001:db8::2", TTL: 600})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_DeleteRecord 验证 DELETE 删除指定记录。
func TestClient_DeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/zones" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]Zone{
				{ID: "zone1", Name: "example.com"},
			})
			return
		}
		if r.URL.Path == "/api/v2/zones/zone1/records/rec1" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "www.example.com", ID: "rec1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_ZoneNotFound 验证 zone 不存在时 GetRecords 报错。
func TestClient_ZoneNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Zone{})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	_, err := client.GetRecords(t.Context(), "unknown.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestClient_ApiError 验证 HTTP 403 时 GetRecords 返回含状态码的错误。
func TestClient_ApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	_, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected status 403 in error, got: %v", err)
	}
}
