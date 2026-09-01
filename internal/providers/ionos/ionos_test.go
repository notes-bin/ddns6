package ionos

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// TestClient 表驱动验证 Client CRUD 路径。
func TestClient(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		run     func(t *testing.T, c *Client)
	}{
		{
			name: "GetRecords",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-API-Key") != "prefix.secret" {
					t.Error("missing X-API-Key header")
				}
				switch {
				case r.URL.Path == "/dns/v1/zones":
					json.NewEncoder(w).Encode([]zone{{ID: "zone1", Name: "example.com"}})
				case strings.HasPrefix(r.URL.Path, "/dns/v1/zones/zone1"):
					json.NewEncoder(w).Encode(zone{
						ID:   "zone1",
						Name: "example.com",
						Records: []record{{
							ID: "rec1", Name: "www.example.com", Type: "AAAA", Content: "2001:db8::1", TTL: 600,
						}},
					})
				default:
					http.NotFound(w, r)
				}
			},
			run: func(t *testing.T, c *Client) {
				records, err := c.GetRecords(t.Context(), "www.example.com", "AAAA")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(records) != 1 || records[0].ID != "rec1" {
					t.Fatalf("unexpected records: %+v", records)
				}
			},
		},
		{
			name: "AddRecord",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusCreated)
					return
				}
				json.NewEncoder(w).Encode([]zone{{ID: "zone1", Name: "example.com"}})
			},
			run: func(t *testing.T, c *Client) {
				err := c.AddRecord(t.Context(), ddns.RecordInfo{
					Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600,
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "DeleteRecord",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusOK)
					return
				}
				json.NewEncoder(w).Encode([]zone{{ID: "zone1", Name: "example.com"}})
			},
			run: func(t *testing.T, c *Client) {
				err := c.DeleteRecord(t.Context(), ddns.RecordInfo{
					Name: "www.example.com", Zone: "example.com", ID: "rec1",
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()
			client := NewClient("prefix", "secret", WithBaseURL(server.URL+"/dns/v1"))
			tt.run(t, client)
		})
	}
}
