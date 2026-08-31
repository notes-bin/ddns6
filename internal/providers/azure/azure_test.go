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

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := NewClient("sub", "tenant", "app", "secret", WithManagementBase(server.URL))
	c.token = "tok"
	c.tokenExpiry = time.Now().Add(time.Hour)
	return c
}

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

func TestExtractResourceGroup(t *testing.T) {
	id := "/subscriptions/sub/resourceGroups/my-rg/providers/Microsoft.Network/dnsZones/example.com"
	if got := extractResourceGroup(id); got != "my-rg" {
		t.Fatalf("expected my-rg, got %s", got)
	}
}

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
