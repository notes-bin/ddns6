package azure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// newTestClient 创建带 mock handler 的测试客户端（预置有效 token）。
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := NewClient("sub", "tenant", "app", "secret", WithManagementBase(server.URL))
	c.token = "tok"
	c.tokenExpiry = time.Now().Add(time.Hour)
	return c
}

// defaultHandler 提供 Azure DNS 常见 API 的默认 mock 响应。
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.Contains(r.URL.Path, "dnsZones") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "AAAA"):
		json.NewEncoder(w).Encode(zoneList{Value: []dnsZone{{
			ID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/dnsZones/example.com",
			Name: "example.com",
		}}})
	case strings.Contains(r.URL.Path, "AAAA") && r.Method == http.MethodGet:
		json.NewEncoder(w).Encode(recordSet{Properties: struct {
			TTL         int          `json:"ttl"`
			AAAARecords []aaaaRecord `json:"aaaaRecords"`
			ARecords    []struct {
				IPv4Address string `json:"ipv4Address"`
			} `json:"aRecords"`
		}{TTL: 600, AAAARecords: []aaaaRecord{{IPv6Address: "2001:db8::1"}}}})
	case strings.Contains(r.URL.Path, "AAAA") && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

// TestExtractResourceGroup 验证 extractResourceGroup 从 ARM ID 提取资源组。
func TestExtractResourceGroup(t *testing.T) {
	id := "/subscriptions/sub/resourceGroups/my-rg/providers/Microsoft.Network/dnsZones/example.com"
	if got := extractResourceGroup(id); got != "my-rg" {
		t.Fatalf("expected my-rg, got %s", got)
	}
}

// TestClient_GetRecords 验证 GetRecords 解析 AAAA 记录。
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

// TestClient_AddRecord 验证 AddRecord 使用 PUT upsert。
func TestClient_AddRecord(t *testing.T) {
	var method string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "AAAA") && r.Method == http.MethodPut {
			method = r.Method
			w.WriteHeader(http.StatusOK)
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
	if method != http.MethodPut {
		t.Errorf("expected PUT, got %s", method)
	}
}

// TestClient_ModifyRecord 验证 ModifyRecord 使用 PUT upsert。
func TestClient_ModifyRecord(t *testing.T) {
	var method string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "AAAA") && r.Method == http.MethodPut {
			method = r.Method
			w.WriteHeader(http.StatusOK)
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
	if method != http.MethodPut {
		t.Errorf("expected PUT, got %s", method)
	}
}

// TestClient_DeleteRecord 验证 DeleteRecord 发送 DELETE 请求。
func TestClient_DeleteRecord(t *testing.T) {
	var method string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "AAAA") && r.Method == http.MethodDelete {
			method = r.Method
			w.WriteHeader(http.StatusOK)
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
	if method != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", method)
	}
}

// TestClient_ApiError 验证非 2xx 响应的错误透传。
func TestClient_ApiError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "dnsZones") {
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

// TestClient_AccessToken 验证 OAuth2 令牌获取与缓存。
func TestClient_AccessToken(t *testing.T) {
	var gotGrant, gotClientID string
	login := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotGrant = r.Form.Get("grant_type")
		gotClientID = r.Form.Get("client_id")
		if !strings.Contains(r.URL.Path, "/tenant/oauth2/v2.0/token") {
			t.Errorf("unexpected token path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}{AccessToken: "fresh-token", ExpiresIn: 3600})
	}))
	t.Cleanup(login.Close)

	mgmt := httptest.NewServer(http.HandlerFunc(defaultHandler))
	t.Cleanup(mgmt.Close)

	c := NewClient("sub", "tenant", "app", "secret",
		WithLoginBase(login.URL),
		WithManagementBase(mgmt.URL),
	)
	records, err := c.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("unexpected records: %+v", records)
	}
	if gotGrant != "client_credentials" || gotClientID != "app" {
		t.Errorf("unexpected token request: grant=%q client_id=%q", gotGrant, gotClientID)
	}
	if c.token != "fresh-token" {
		t.Errorf("expected cached token fresh-token, got %q", c.token)
	}

	// 缓存未过期时不应再请求登录端点
	login.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not refresh token when still valid")
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})
	if _, err := c.GetRecords(t.Context(), "www.example.com", "AAAA"); err != nil {
		t.Fatalf("cached token path failed: %v", err)
	}
}

// TestClient_AccessTokenError 验证令牌请求失败时的错误处理。
func TestClient_AccessTokenError(t *testing.T) {
	login := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(login.Close)

	c := NewClient("sub", "tenant", "app", "secret", WithLoginBase(login.URL), WithManagementBase("http://127.0.0.1:1"))
	_, err := c.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}
