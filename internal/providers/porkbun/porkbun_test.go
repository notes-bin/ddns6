package porkbun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_GetRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/retrieveByNameType/example.com/AAAA/www") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// 验证认证信息
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["apikey"] != "test-key" || body["secretapikey"] != "test-secret" {
			t.Error("missing API credentials in body")
		}
		json.NewEncoder(w).Encode(apiResponse{
			Status: "SUCCESS",
			Records: []DNSRecord{
				{Name: "www", Type: "AAAA", Content: "2001:db8::1", TTL: "600"},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
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
}

func TestClient_AddRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/create/example.com") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(apiResponse{Status: "SUCCESS"})
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "www.example.com", "AAAA", "2001:db8::1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/editByNameType/example.com/AAAA/www") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(apiResponse{Status: "SUCCESS"})
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
	err := client.ModifyRecord(context.Background(), "www.example.com", "", "AAAA", "2001:db8::2", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/deleteByNameType/example.com/AAAA/www") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(apiResponse{Status: "SUCCESS"})
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
	err := client.DeleteRecord(context.Background(), "www.example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiResponse{Status: "ERROR"})
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "www.example.com", "AAAA", "2001:db8::1", 600)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSplitDomain(t *testing.T) {
	tests := []struct {
		input      string
		wantDomain string
		wantSub    string
	}{
		{"example.com", "example.com", "@"},
		{"www.example.com", "example.com", "www"},
		{"sub.www.example.com", "example.com", "sub.www"},
	}
	for _, tt := range tests {
		domain, sub := splitDomain(tt.input)
		if domain != tt.wantDomain || sub != tt.wantSub {
			t.Errorf("splitDomain(%q) = (%q, %q), want (%q, %q)",
				tt.input, domain, sub, tt.wantDomain, tt.wantSub)
		}
	}
}
