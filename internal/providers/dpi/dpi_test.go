package dpi

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
				if !strings.Contains(r.PostFormValue("login_token"), "id,key") {
					t.Error("missing login_token")
				}
				json.NewEncoder(w).Encode(recordListResponse{
					Status: apiStatus{Code: "1", Message: "success"},
					Records: []record{{
						ID: 1, Name: "www", Type: "AAAA", Value: "2001:db8::1", TTL: "600",
					}},
				})
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
				json.NewEncoder(w).Encode(recordResponse{Status: apiStatus{Code: "1", Message: "success"}})
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				tt.handler(w, r)
			}))
			defer server.Close()
			client := NewClient("id,key", WithBaseURL(server.URL))
			tt.run(t, client)
		})
	}
}
