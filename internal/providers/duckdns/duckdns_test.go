package duckdns

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// TestClient_AddRecord 验证 DuckDNS 更新请求的 query 参数与成功响应。
func TestClient_AddRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("token") != "test-token" {
			t.Errorf("expected token test-token, got %s", r.URL.Query().Get("token"))
		}
		if r.URL.Query().Get("domains") != "myhost" {
			t.Errorf("expected domains myhost, got %s", r.URL.Query().Get("domains"))
		}
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "myhost.duckdns.org", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_ModifyRecord 验证 ModifyRecord 通过 ipv6 参数更新地址。
func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ipv6") != "2001:db8::2" {
			t.Errorf("expected ipv6 2001:db8::2, got %s", r.URL.Query().Get("ipv6"))
		}
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "myhost.duckdns.org", ID: "", Type: "AAAA", Value: "2001:db8::2", TTL: 600})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_DeleteRecord 验证 DeleteRecord 发送空 ipv6 清除记录。
func TestClient_DeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ipv6") != "" {
			t.Errorf("expected empty ipv6, got %s", r.URL.Query().Get("ipv6"))
		}
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "myhost.duckdns.org", ID: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_GetRecords_Empty 验证受限 API 下 GetRecords 恒返回空列表。
func TestClient_GetRecords_Empty(t *testing.T) {
	client := NewClient("test-token")
	records, err := client.GetRecords(t.Context(), "myhost.duckdns.org", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty records, got %d", len(records))
	}
}

// TestClient_APIError 验证 API 返回 KO 时 AddRecord 报错。
func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("KO"))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "myhost.duckdns.org", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
