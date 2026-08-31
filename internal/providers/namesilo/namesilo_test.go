package namesilo

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

func TestClient(t *testing.T) {
	const listDomainsOK = `<namesilo><reply><code>300</code><detail>success</detail><domains><domain>example.com</domain><domain>other.net</domain></domains></reply></namesilo>`
	const listRecordsOK = `<namesilo><reply><code>300</code><detail>success</detail><resource_record><record_id>1</record_id><type>AAAA</type><host>www</host><value>2001:db8::1</value><ttl>600</ttl></resource_record></reply></namesilo>`
	const actionOK = `<namesilo><reply><code>300</code><detail>success</detail></reply></namesilo>`

	tests := []struct {
		name    string
		handler http.HandlerFunc
		run     func(t *testing.T, c *Client)
	}{
		{
			name: "GetRecords",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "listDomains"):
					fmt.Fprint(w, listDomainsOK)
				case strings.Contains(r.URL.Path, "dnsListRecords"):
					fmt.Fprint(w, listRecordsOK)
				default:
					http.NotFound(w, r)
				}
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
				fmt.Fprint(w, actionOK)
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
				fmt.Fprint(w, actionOK)
			},
			run: func(t *testing.T, c *Client) {
				err := c.DeleteRecord(t.Context(), ddns.RecordInfo{
					Name: "www.example.com", Zone: "example.com", ID: "1",
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "findZone structured parse",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, listDomainsOK)
			},
			run: func(t *testing.T, c *Client) {
				zone, sub, err := c.findZone(t.Context(), "www.other.net")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if zone != "other.net" || sub != "www" {
					t.Fatalf("unexpected zone/sub: %s / %s", zone, sub)
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
