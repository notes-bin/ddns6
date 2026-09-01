package gcloud

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// newTestClient 创建带 mock handler 的测试客户端。
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewClient("proj", "test-token", WithBaseURL(server.URL))
}

// defaultHandler 提供 Cloud DNS 常见 API 的默认 mock 响应。
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer test-token" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case strings.Contains(r.URL.Path, "/rrsets"):
		json.NewEncoder(w).Encode(map[string]any{
			"rrsets": []rrSet{{Name: "www.example.com.", Type: "AAAA", TTL: 600, Rrdatas: []string{"2001:db8::1"}}},
		})
	case strings.Contains(r.URL.Path, "/managedZones") && r.Method == http.MethodGet:
		json.NewEncoder(w).Encode(zoneList{
			ManagedZones: []managedZone{{Name: "example-com", DNSName: "example.com."}},
		})
	case strings.Contains(r.URL.Path, "/changes") && r.Method == http.MethodPost:
		json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
	default:
		http.NotFound(w, r)
	}
}

// TestClient_GetRecords 验证 GetRecords 解析 rrsets。
func TestClient_GetRecords(t *testing.T) {
	client := newTestClient(t, defaultHandler)
	records, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 || records[0].Value != "2001:db8::1" {
		t.Fatalf("unexpected records: %+v", records)
	}
}

// TestClient_AddRecord 验证 AddRecord 提交 additions 变更。
func TestClient_AddRecord(t *testing.T) {
	var hasAddition bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/changes") && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			hasAddition = strings.Contains(string(body), `"additions"`)
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
			return
		}
		defaultHandler(w, r)
	})
	err := client.AddRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::2", TTL: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAddition {
		t.Error("expected additions in change request")
	}
}

// TestClient_ModifyRecord 验证 ModifyRecord 先删后增。
func TestClient_ModifyRecord(t *testing.T) {
	var changeCount int
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/changes") && r.Method == http.MethodPost {
			changeCount++
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
			return
		}
		defaultHandler(w, r)
	})
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::3", TTL: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changeCount != 2 {
		t.Errorf("ModifyRecord should call changes twice (delete+add), got %d", changeCount)
	}
}

// TestClient_DeleteRecord 验证 DeleteRecord 提交 deletions 变更。
func TestClient_DeleteRecord(t *testing.T) {
	var hasDeletion bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/changes") && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			hasDeletion = strings.Contains(string(body), `"deletions"`)
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
			return
		}
		defaultHandler(w, r)
	})
	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDeletion {
		t.Error("expected deletions in change request")
	}
}

// TestClient_ApiError 验证非 2xx 响应的错误透传。
func TestClient_ApiError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/managedZones") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	})
	_, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected status 403 in error, got: %v", err)
	}
}
