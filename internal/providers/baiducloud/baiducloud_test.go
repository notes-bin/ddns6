package baiducloud

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
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/domain/resolve/list") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(baiduListResponse{
			Result: []struct {
				RecordID string `json:"recordId"`
				Domain   string `json:"domain"`
				RDType   string `json:"rdtype"`
				RData    string `json:"rdata"`
				TTL      int    `json:"ttl"`
				View     string `json:"view"`
				ZoneName string `json:"zoneName"`
			}{
				{RecordID: "rec1", Domain: "www", RDType: "AAAA", RData: "2001:db8::1", TTL: 300, View: "default", ZoneName: "example.com"},
			},
			TotalCount: 1,
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
		if !strings.HasSuffix(r.URL.Path, "/v1/domain/resolve/add") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "www.example.com", "AAAA", "2001:db8::1", 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ModifyRecord(t *testing.T) {
	var listCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/v1/domain/resolve/list") {
			listCalled = true
			json.NewEncoder(w).Encode(baiduListResponse{
				Result: []struct {
					RecordID string `json:"recordId"`
					Domain   string `json:"domain"`
					RDType   string `json:"rdtype"`
					RData    string `json:"rdata"`
					TTL      int    `json:"ttl"`
					View     string `json:"view"`
					ZoneName string `json:"zoneName"`
				}{
					{RecordID: "rec1", Domain: "www", RDType: "AAAA", RData: "2001:db8::2", TTL: 300, View: "default", ZoneName: "example.com"},
				},
				TotalCount: 1,
			})
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/domain/resolve/edit") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
	err := client.ModifyRecord(context.Background(), "www.example.com", "rec1", "AAAA", "2001:db8::2", 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !listCalled {
		t.Error("expected list endpoint to be called for View lookup")
	}
}

func TestClient_DeleteRecord_Noop(t *testing.T) {
	client := NewClient("test-key", "test-secret")
	err := client.DeleteRecord(context.Background(), "www.example.com", "rec1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSplitDomain(t *testing.T) {
	_, sub, zone := splitDomain("www.example.com")
	if sub != "www" {
		t.Errorf("expected sub www, got %s", sub)
	}
	if zone != "example.com" {
		t.Errorf("expected zone example.com, got %s", zone)
	}
}
