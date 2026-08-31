package aws

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

func TestClient(t *testing.T) {
	const zonesXML = `<?xml version="1.0"?><ListHostedZonesResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><HostedZones><HostedZone><Id>/hostedzone/Z123</Id><Name>example.com.</Name></HostedZone></HostedZones></ListHostedZonesResponse>`
	const changeOK = `<?xml version="1.0"?><ChangeResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><ChangeInfo/></ChangeResourceRecordSetsResponse>`
	const rrsetXML = `<?xml version="1.0"?><ListResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><ResourceRecordSets><ResourceRecordSet><Name>www.example.com.</Name><Type>AAAA</Type><TTL>600</TTL><ResourceRecords><ResourceRecord><Value>2001:db8::1</Value></ResourceRecord></ResourceRecords></ResourceRecordSet></ResourceRecordSets></ListResourceRecordSetsResponse>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/rrset") && r.Method == http.MethodGet:
			fmt.Fprint(w, rrsetXML)
		case strings.Contains(r.URL.Path, "hostedzone") && r.Method == http.MethodGet:
			fmt.Fprint(w, zonesXML)
		case strings.Contains(r.URL.Path, "rrset") && r.Method == http.MethodPost:
			fmt.Fprint(w, changeOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	client := NewClient("AKID", "SECRET", WithHost(u.Host), WithScheme(u.Scheme))

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
