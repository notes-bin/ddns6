package aws

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

const (
	testZonesXML = `<?xml version="1.0"?><ListHostedZonesResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><HostedZones><HostedZone><Id>/hostedzone/Z123</Id><Name>example.com.</Name></HostedZone></HostedZones></ListHostedZonesResponse>`
	testChangeOK = `<?xml version="1.0"?><ChangeResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><ChangeInfo/></ChangeResourceRecordSetsResponse>`
	testRRSetXML = `<?xml version="1.0"?><ListResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><ResourceRecordSets><ResourceRecordSet><Name>www.example.com.</Name><Type>AAAA</Type><TTL>600</TTL><ResourceRecords><ResourceRecord><Value>2001:db8::1</Value></ResourceRecord></ResourceRecords></ResourceRecordSet></ResourceRecordSets></ListResourceRecordSetsResponse>`
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return NewClient("AKID", "SECRET", WithHost(u.Host), WithScheme(u.Scheme))
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.Contains(r.URL.Path, "/rrset") && r.Method == http.MethodGet:
		fmt.Fprint(w, testRRSetXML)
	case strings.Contains(r.URL.Path, "hostedzone") && r.Method == http.MethodGet:
		fmt.Fprint(w, testZonesXML)
	case strings.Contains(r.URL.Path, "rrset") && r.Method == http.MethodPost:
		fmt.Fprint(w, testChangeOK)
	default:
		http.NotFound(w, r)
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
	var action string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "rrset") && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "<Action>UPSERT</Action>") {
				action = "UPSERT"
			}
			fmt.Fprint(w, testChangeOK)
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
	if action != "UPSERT" {
		t.Errorf("expected UPSERT action, got %q", action)
	}
}

func TestClient_ModifyRecord(t *testing.T) {
	var action string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "rrset") && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "<Action>UPSERT</Action>") {
				action = "UPSERT"
			}
			fmt.Fprint(w, testChangeOK)
			return
		}
		defaultHandler(w, r)
	})
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", ID: "1", Type: "AAAA", Value: "2001:db8::3", TTL: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != "UPSERT" {
		t.Errorf("expected UPSERT action, got %q", action)
	}
}

func TestClient_DeleteRecord(t *testing.T) {
	var action string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "rrset") && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "<Action>DELETE</Action>") {
				action = "DELETE"
			}
			fmt.Fprint(w, testChangeOK)
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
	if action != "DELETE" {
		t.Errorf("expected DELETE action, got %q", action)
	}
}

func TestClient_ApiError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "hostedzone") {
			http.Error(w, "AccessDenied", http.StatusForbidden)
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
