package baiducloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// TestClient_GetRecords 验证 resolve/list 解析与 BCE 签名头。
func TestClient_GetRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/domain/resolve/list") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(baiduListResponse{
			Result: []DNSRecord{
				{RecordID: "rec1", Domain: "www", RDType: "AAAA", RData: "2001:db8::1", TTL: 300, View: "default", ZoneName: "example.com"},
			},
			TotalCount: 1,
		})
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
	records, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
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

// TestClient_AddRecord 验证 resolve/add 成功路径。
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
	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 300})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_ModifyRecord 验证修改前会查询 view 字段。
func TestClient_ModifyRecord(t *testing.T) {
	var listCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/v1/domain/resolve/list") {
			listCalled = true
			json.NewEncoder(w).Encode(baiduListResponse{
				Result: []DNSRecord{
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
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "www.example.com", ID: "rec1", Type: "AAAA", Value: "2001:db8::2", TTL: 300})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !listCalled {
		t.Error("expected list endpoint to be called for View lookup")
	}
}

// TestClient_DeleteRecord 验证 resolve/delete 成功路径。
func TestClient_DeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/domain/resolve/delete") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-key", "test-secret", WithBaseURL(server.URL))
	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "www.example.com", ID: "rec1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
