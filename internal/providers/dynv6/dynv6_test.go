package dynv6

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetRecords_Zone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 匹配 zone 列表请求
		if r.URL.Path == "/api/v2/zones" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]Zone{
				{ID: "zone1", Name: "example.com", IPv6: "2001:db8::1"},
			})
			return
		}
		// 匹配 zone 详情请求
		if r.URL.Path == "/api/v2/zones/zone1" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(Zone{ID: "zone1", Name: "example.com", IPv6: "2001:db8::1"})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	records, err := client.GetRecords(context.Background(), "example.com", "AAAA")
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
	err := client.AddRecord(context.Background(), "www.example.com", "AAAA", "2001:db8::1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

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
	err := client.ModifyRecord(context.Background(), "www.example.com", "rec1", "AAAA", "2001:db8::2", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

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
	err := client.DeleteRecord(context.Background(), "www.example.com", "rec1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ZoneNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Zone{})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))
	_, err := client.GetRecords(context.Background(), "unknown.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
