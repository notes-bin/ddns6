package gcloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

func TestClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing bearer token")
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
		case strings.Contains(r.URL.Path, "/changes"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("proj", "test-token", WithBaseURL(server.URL))

	t.Run("GetRecords", func(t *testing.T) {
		records, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 1 || records[0].Value != "2001:db8::1" {
			t.Fatalf("unexpected records: %+v", records)
		}
	})

	t.Run("AddRecord", func(t *testing.T) {
		err := client.AddRecord(t.Context(), ddns.RecordInfo{
			Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::2", TTL: 600,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
