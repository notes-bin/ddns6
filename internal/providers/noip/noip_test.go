package noip

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	err := client.AddRecord(context.Background(), "myhost.example.com", "AAAA", "2001:db8::1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("good 2001:db8::2"))
	}))
	defer server.Close()

	client := NewClient("user", "pass", WithBaseURL(server.URL))
	err := client.ModifyRecord(context.Background(), "myhost.example.com", "", "AAAA", "2001:db8::2", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Nochg(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("nochg 2001:db8::1"))
	}))
	defer server.Close()

	client := NewClient("user", "pass", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "myhost.example.com", "AAAA", "2001:db8::1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_GetRecords_Empty(t *testing.T) {
	client := NewClient("user", "pass")
	records, err := client.GetRecords(context.Background(), "myhost.example.com", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty records, got %d", len(records))
	}
}

func TestClient_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("badauth"))
	}))
	defer server.Close()

	client := NewClient("user", "wrongpass", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "myhost.example.com", "AAAA", "2001:db8::1", 600)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestClient_Nohost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("nohost"))
	}))
	defer server.Close()

	client := NewClient("user", "pass", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "nonexistent.example.com", "AAAA", "2001:db8::1", 600)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
