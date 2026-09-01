package noip

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// TestClient_AddRecord 验证 No-IP DDNS 的 Basic Auth 与 myip 参数。
func TestClient_AddRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		// 验证 Basic Auth
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Error("expected Basic Auth")
		}
		decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		if string(decoded) != "user:pass" {
			t.Errorf("expected user:pass, got %s", decoded)
		}
		if r.URL.Query().Get("hostname") != "myhost.example.com" {
			t.Errorf("expected hostname myhost.example.com, got %s", r.URL.Query().Get("hostname"))
		}
		if r.URL.Query().Get("myip") != "2001:db8::1" {
			t.Errorf("expected myip 2001:db8::1, got %s", r.URL.Query().Get("myip"))
		}
		w.Write([]byte("good 2001:db8::1"))
	}))
	defer server.Close()

	client := NewClient("user", "pass", WithBaseURL(server.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "myhost.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_ModifyRecord 验证 ModifyRecord 成功更新 IPv6。
func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("good 2001:db8::2"))
	}))
	defer server.Close()

	client := NewClient("user", "pass", WithBaseURL(server.URL))
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "myhost.example.com", ID: "", Type: "AAAA", Value: "2001:db8::2", TTL: 600})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_Nochg 验证 API 返回 nochg 时 AddRecord 视为成功。
func TestClient_Nochg(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("nochg 2001:db8::1"))
	}))
	defer server.Close()

	client := NewClient("user", "pass", WithBaseURL(server.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "myhost.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_GetRecords_Empty 验证受限 API 下 GetRecords 恒返回空列表。
func TestClient_GetRecords_Empty(t *testing.T) {
	client := NewClient("user", "pass")
	records, err := client.GetRecords(t.Context(), "myhost.example.com", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty records, got %d", len(records))
	}
}

// TestClient_AuthError 验证 badauth 响应时 AddRecord 报错。
func TestClient_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("badauth"))
	}))
	defer server.Close()

	client := NewClient("user", "wrongpass", WithBaseURL(server.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "myhost.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestClient_Nohost 验证 nohost 响应时 AddRecord 报错。
func TestClient_Nohost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("nohost"))
	}))
	defer server.Close()

	client := NewClient("user", "pass", WithBaseURL(server.URL))
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "nonexistent.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
