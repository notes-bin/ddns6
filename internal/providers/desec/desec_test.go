package desec

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

func TestClient(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		run     func(t *testing.T, c *Client)
	}{
		{
			name: "GetRecords",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Token test-token" {
					t.Error("missing Authorization header")
				}
				switch {
				case r.URL.Path == "/api/v1/domains/":
					json.NewEncoder(w).Encode([]domainInfo{{Name: "example.com"}})
				case strings.Contains(r.URL.Path, "/rrsets/www/AAAA/"):
					json.NewEncoder(w).Encode(rrset{
						Subname: "www", Type: "AAAA", TTL: 3600, Records: []string{"2001:db8::1"},
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
				if len(records) != 1 || records[0].Value != "2001:db8::1" {
					t.Fatalf("unexpected records: %+v", records)
				}
			},
		},
		{
			name: "AddRecord",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT, got %s", r.Method)
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode([]rrset{{Subname: "www", Type: "AAAA", Records: []string{"2001:db8::1"}}})
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
			name: "ModifyRecord",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rrsets/www/AAAA/"):
					json.NewEncoder(w).Encode(rrset{
						Subname: "www", Type: "AAAA", TTL: 3600, Records: []string{"2001:db8::1"},
					})
				case r.Method == http.MethodPut:
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode([]rrset{{Subname: "www", Type: "AAAA", Records: []string{"2001:db8::2"}}})
				default:
					http.NotFound(w, r)
				}
			},
			run: func(t *testing.T, c *Client) {
				err := c.ModifyRecord(t.Context(), ddns.RecordInfo{
					Name: "www.example.com", Zone: "example.com", ID: "www|2001:db8::1",
					Type: "AAAA", Value: "2001:db8::2", TTL: 600,
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "ApiError",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			},
			run: func(t *testing.T, c *Client) {
				_, err := c.GetRecords(t.Context(), "www.example.com", "AAAA")
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "403") {
					t.Errorf("expected status 403 in error, got: %v", err)
				}
			},
		},
		{
			name: "DeleteRecord",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					json.NewEncoder(w).Encode(rrset{Subname: "www", Type: "AAAA", Records: []string{"2001:db8::1"}})
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			run: func(t *testing.T, c *Client) {
				err := c.DeleteRecord(t.Context(), ddns.RecordInfo{
					Name: "www.example.com", Zone: "example.com", ID: "www|2001:db8::1", Type: "AAAA", Value: "2001:db8::1",
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
			client := NewClient("test-token", WithBaseURL(server.URL+"/api/v1"))
			tt.run(t, client)
		})
	}
}
