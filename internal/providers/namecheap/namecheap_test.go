package namecheap

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

func TestClient(t *testing.T) {
	const getHostsOK = `<?xml version="1.0"?><ApiResponse Status="OK"><CommandResponse><DomainDNSGetHostsResult Domain="example.com"><host HostId="1" Name="www" Type="AAAA" Address="2001:db8::1" MXPref="10" TTL="600"/></DomainDNSGetHostsResult></CommandResponse></ApiResponse>`
	const setHostsOK = `<?xml version="1.0"?><ApiResponse Status="OK"><CommandResponse/></ApiResponse>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.RawQuery, "getHosts"):
			fmt.Fprint(w, getHostsOK)
		case strings.Contains(r.URL.RawQuery, "setHosts"):
			fmt.Fprint(w, setHostsOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("key", "user", "127.0.0.1", WithBaseURL(server.URL))

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
