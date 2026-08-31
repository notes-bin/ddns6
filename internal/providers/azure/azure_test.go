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

func TestExtractResourceGroup(t *testing.T) {
	id := "/subscriptions/sub/resourceGroups/my-rg/providers/Microsoft.Network/dnsZones/example.com"
	if got := extractResourceGroup(id); got != "my-rg" {
		t.Fatalf("expected my-rg, got %s", got)
	}
}

func TestClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		case strings.Contains(r.URL.Path, "AAAA") && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("sub", "tenant", "app", "secret", WithManagementBase(server.URL))
	c.token = "tok"
	c.tokenExpiry = time.Now().Add(time.Hour)

	t.Run("GetRecords", func(t *testing.T) {
		records, err := c.GetRecords(t.Context(), "www.example.com", "AAAA")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 1 || records[0].Value != "2001:db8::1" {
			t.Fatalf("unexpected records: %+v", records)
		}
	})

	t.Run("AddRecord", func(t *testing.T) {
		err := c.AddRecord(t.Context(), ddns.RecordInfo{
			Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::2", TTL: 600,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
