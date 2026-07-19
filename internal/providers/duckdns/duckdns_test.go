package duckdns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	err := client.AddRecord(context.Background(), "myhost.duckdns.org", "AAAA", "2001:db8::1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ipv6") != "2001:db8::2" {
			t.Errorf("expected ipv6 2001:db8::2, got %s", r.URL.Query().Get("ipv6"))
		}
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.ModifyRecord(context.Background(), "myhost.duckdns.org", "", "AAAA", "2001:db8::2", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ipv6") != "" {
			t.Errorf("expected empty ipv6, got %s", r.URL.Query().Get("ipv6"))
		}
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.DeleteRecord(context.Background(), "myhost.duckdns.org", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_GetRecords_Empty(t *testing.T) {
	client := NewClient("test-token")
	records, err := client.GetRecords(context.Background(), "myhost.duckdns.org", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty records, got %d", len(records))
	}
}

func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("KO"))
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "myhost.duckdns.org", "AAAA", "2001:db8::1", 600)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
