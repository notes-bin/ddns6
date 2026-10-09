package gcloud

import (
	"encoding/json"
	"errors"
	"fmt"
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

// TestClient_ModifyRecord 验证 ModifyRecord 单次 Change 同时含 deletions 与 additions。
func TestClient_ModifyRecord(t *testing.T) {
	var changeCount int
	var hasDeletion, hasAddition bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/changes") && r.Method == http.MethodPost {
			changeCount++
			body, _ := io.ReadAll(r.Body)
			hasDeletion = strings.Contains(string(body), `"deletions"`)
			hasAddition = strings.Contains(string(body), `"additions"`)
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
			return
		}
		defaultHandler(w, r)
	})
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", ID: "www.example.com.|AAAA|2001:db8::1",
		Type: "AAAA", Value: "2001:db8::3", TTL: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changeCount != 1 {
		t.Errorf("ModifyRecord should call changes once (atomic replace), got %d", changeCount)
	}
	if !hasDeletion || !hasAddition {
		t.Errorf("expected both deletions and additions in one change, deletion=%v addition=%v", hasDeletion, hasAddition)
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

// TestHTTPStatusError_Error 验证错误字符串包含状态码与正文。
func TestHTTPStatusError_Error(t *testing.T) {
	t.Parallel()
	err := &httpStatusError{status: http.StatusBadRequest, body: "bad"}
	got := err.Error()
	if !strings.Contains(got, "400") || !strings.Contains(got, "bad") {
		t.Fatalf("Error() = %q", got)
	}
}

// TestIsNotFound 覆盖 404、非 404 与非 httpStatusError。
func TestIsNotFound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "404", err: &httpStatusError{status: http.StatusNotFound}, want: true},
		{name: "403", err: &httpStatusError{status: http.StatusForbidden}, want: false},
		{name: "普通错误", err: errors.New("boom"), want: false},
		{name: "nil", err: nil, want: false},
		{name: "包装 404", err: fmt.Errorf("wrap: %w", &httpStatusError{status: http.StatusNotFound}), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isNotFound(tt.err); got != tt.want {
				t.Errorf("isNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}
