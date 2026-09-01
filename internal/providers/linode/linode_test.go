package linode

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
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing Authorization header")
				}
				if strings.Contains(r.URL.Path, "/123/records") {
					json.NewEncoder(w).Encode(map[string]any{
						"data": []domainRecord{{ID: 1, Type: "AAAA", Name: "www", Target: "2001:db8::1", TTL: 600}},
					})
					return
				}
				if r.Header.Get("X-Filter") != "" {
					json.NewEncoder(w).Encode(map[string]any{
						"data": []domain{{ID: 123, Domain: "example.com"}},
					})
					return
				}
				http.NotFound(w, r)
			},
			run: func(t *testing.T, c *Client) {
				records, err := c.GetRecords(t.Context(), "www.example.com", "AAAA")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(records) != 1 || records[0].ID != "1" {
					t.Fatalf("unexpected records: %+v", records)
				}
			},
		},
		{
			name: "AddRecord",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/records") {
					w.WriteHeader(http.StatusCreated)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{
					"data": []domain{{ID: 123, Domain: "example.com"}},
				})
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
				if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/records/") {
					w.WriteHeader(http.StatusOK)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{
					"data": []domain{{ID: 123, Domain: "example.com"}},
				})
			},
			run: func(t *testing.T, c *Client) {
				err := c.ModifyRecord(t.Context(), ddns.RecordInfo{
					Name: "www.example.com", Zone: "example.com", ID: "42", Type: "AAAA", Value: "2001:db8::2", TTL: 600,
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
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{
					"data": []domain{{ID: 123, Domain: "example.com"}},
				})
			},
			run: func(t *testing.T, c *Client) {
				err := c.DeleteRecord(t.Context(), ddns.RecordInfo{
					Name: "www.example.com", Zone: "example.com", ID: "42",
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
			client := NewClient("test-key", WithBaseURL(server.URL))
			tt.run(t, client)
		})
	}
}
