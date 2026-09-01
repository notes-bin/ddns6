package dpi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// newTestClient 创建指向 mock 服务器的客户端，并自动 ParseForm。
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return NewClient("id,key", WithBaseURL(server.URL))
}

// TestClient_GetRecords 验证 Record.List 解析与认证参数。
func TestClient_GetRecords(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.PostFormValue("login_token"), "id,key") {
			t.Error("missing login_token")
		}
		if !strings.HasSuffix(r.URL.Path, "/Record.List") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(recordListResponse{
			Status:  apiStatus{Code: "1", Message: "success"},
			Records: []record{{ID: 1, Name: "www", Type: "AAAA", Value: "2001:db8::1", TTL: "600"}},
		})
	})
	records, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 || records[0].ID != "1" {
		t.Fatalf("unexpected records: %+v", records)
	}
}

// TestClient_AddRecord 验证 Record.Create 成功路径。
func TestClient_AddRecord(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/Record.Create") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(recordResponse{Status: apiStatus{Code: "1", Message: "success"}})
	})
	err := client.AddRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_ModifyRecord 验证 Record.Modify 携带 record_id。
func TestClient_ModifyRecord(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/Record.Modify") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.PostFormValue("record_id") != "123" {
			t.Errorf("expected record_id 123, got %q", r.PostFormValue("record_id"))
		}
		json.NewEncoder(w).Encode(recordResponse{Status: apiStatus{Code: "1", Message: "success"}})
	})
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", ID: "123", Type: "AAAA", Value: "2001:db8::2", TTL: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_DeleteRecord 验证 Record.Remove 携带 record_id。
func TestClient_DeleteRecord(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/Record.Remove") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.PostFormValue("record_id") != "123" {
			t.Errorf("expected record_id 123, got %q", r.PostFormValue("record_id"))
		}
		json.NewEncoder(w).Encode(recordResponse{Status: apiStatus{Code: "1", Message: "success"}})
	})
	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", ID: "123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClient_ApiError 验证业务错误码被正确返回。
func TestClient_ApiError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(recordListResponse{
			Status: apiStatus{Code: "6", Message: "domain banned"},
		})
	})
	_, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "domain banned") {
		t.Errorf("unexpected error: %v", err)
	}
}
