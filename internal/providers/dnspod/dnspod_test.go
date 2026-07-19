package dnspod

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
		if !strings.HasSuffix(r.URL.Path, "/Record.List") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(recordListResponse{
			Status: dnspodStatus{Code: "1", Message: "成功"},
			Records: []dnspodRecord{
				{ID: 123, Name: "www", Type: "AAAA", Value: "2001:db8::1", TTL: "600"},
			},
		})
	}))
	defer server.Close()

	client := NewClient("12345,mytoken", WithBaseURL(server.URL))
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
	if records[0].ID != "123" {
		t.Errorf("expected ID 123, got %s", records[0].ID)
	}
}

func TestClient_AddRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/Record.Create") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(recordResponse{
			Status: dnspodStatus{Code: "1", Message: "成功"},
		})
	}))
	defer server.Close()

	client := NewClient("12345,mytoken", WithBaseURL(server.URL))
	err := client.AddRecord(context.Background(), "www.example.com", "AAAA", "2001:db8::1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ModifyRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/Record.Modify") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(recordResponse{
			Status: dnspodStatus{Code: "1", Message: "成功"},
		})
	}))
	defer server.Close()

	client := NewClient("12345,mytoken", WithBaseURL(server.URL))
	err := client.ModifyRecord(context.Background(), "www.example.com", "123", "AAAA", "2001:db8::2", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/Record.Remove") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(recordResponse{
			Status: dnspodStatus{Code: "1", Message: "成功"},
		})
	}))
	defer server.Close()

	client := NewClient("12345,mytoken", WithBaseURL(server.URL))
	err := client.DeleteRecord(context.Background(), "www.example.com", "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(recordListResponse{
			Status: dnspodStatus{Code: "6", Message: "域名已被封禁"},
		})
	}))
	defer server.Close()

	client := NewClient("12345,mytoken", WithBaseURL(server.URL))
	_, err := client.GetRecords(context.Background(), "www.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSplitDomain(t *testing.T) {
	domain, sub := splitDomain("www.example.com")
	if domain != "example.com" || sub != "www" {
		t.Errorf("splitDomain(www.example.com) = (%s, %s), want (example.com, www)", domain, sub)
	}

	domain, sub = splitDomain("example.com")
	if domain != "example.com" || sub != "@" {
		t.Errorf("splitDomain(example.com) = (%s, %s), want (example.com, @)", domain, sub)
	}
}
