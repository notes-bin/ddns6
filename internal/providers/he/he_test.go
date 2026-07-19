package he

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
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Error("expected Basic Auth")
		}
		decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		user := strings.SplitN(string(decoded), ":", 2)[0]
		if user != "hosted_dns_editapi" {
			t.Errorf("expected user hosted_dns_editapi, got %s", user)
		}
		if r.URL.Query().Get("hostname") != "myhost.example.com" {
			t.Errorf("expected hostname myhost.example.com, got %s", r.URL.Query().Get("hostname"))
		}
		w.Write([]byte("good 2001:db8::1"))
	}))
	defer server.Close()

	client := NewClient("ddns-key", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "myhost.example.com", "AAAA", "2001:db8::1", 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("good 2001:db8::2"))
	}))
	defer server.Close()

	client := NewClient("ddns-key", WithBaseURL(server.URL))
	err := client.ModifyRecord(context.Background(), "myhost.example.com", "", "AAAA", "2001:db8::2", 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Nochg(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("nochg 2001:db8::1"))
	}))
	defer server.Close()

	client := NewClient("ddns-key", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "myhost.example.com", "AAAA", "2001:db8::1", 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_GetRecords_Empty(t *testing.T) {
	client := NewClient("ddns-key")
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

	client := NewClient("wrong-key", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "myhost.example.com", "AAAA", "2001:db8::1", 300)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
