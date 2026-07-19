package digitalocean

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/providers"
)

func TestClient_GetRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing or invalid Authorization header")
		}
		if !strings.HasSuffix(r.URL.Path, "/domains/example.com/records") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"domain_records": []DomainRecord{
				{ID: 1, Type: "AAAA", Name: "www", Data: "2001:db8::1", TTL: 600},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	records, err := client.GetRecords(context.Background(), "www.example.com", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Value != "2001:db8::1" {
		t.Errorf("expected 2001:db8::1, got %s", records[0].Value)
	}
	if records[0].ID != "1" {
		t.Errorf("expected ID 1, got %s", records[0].ID)
	}
}

func TestClient_AddRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"domain_record": DomainRecord{ID: 2}})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "www.example.com", "AAAA", "2001:db8::1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/records/42") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"domain_record": DomainRecord{ID: 42}})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.ModifyRecord(context.Background(), "www.example.com", "42", "AAAA", "2001:db8::2", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	err := client.DeleteRecord(context.Background(), "www.example.com", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
	}))
	defer server.Close()

	client := NewClient("bad-token", WithBaseURL(server.URL))
	_, err := client.GetRecords(context.Background(), "www.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSplitDomain(t *testing.T) {
	tests := []struct {
		input    string
		wantRoot string
		wantSub  string
	}{
		{"example.com", "example.com", "@"},
		{"www.example.com", "example.com", "www"},
		{"sub.www.example.com", "example.com", "sub.www"},
	}
	for _, tt := range tests {
		root, sub := providers.SplitDomain(tt.input)
		if root != tt.wantRoot {
			t.Errorf("SplitDomain(%q) root = %q, want %q", tt.input, root, tt.wantRoot)
		}
		if sub != tt.wantSub {
			t.Errorf("SplitDomain(%q) sub = %q, want %q", tt.input, sub, tt.wantSub)
		}
	}
}
