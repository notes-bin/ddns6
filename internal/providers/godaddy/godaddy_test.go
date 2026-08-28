package godaddy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// newGoDaddyTestServer creates a test server that handles both domain lookup and record operations
func newGoDaddyTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/domains/") && !strings.Contains(r.URL.Path, "/records/") {
			// Domain lookup - return valid JSON
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"domain": "example.com"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `[{"data": "192.168.1.1", "name": "test", "type": "A", "ttl": 600}]`)
		}
	}))
}

func TestAddRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.AddRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", Type: "A", Value: "192.168.1.1", TTL: 600})
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

func TestModifyRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "192.168.1.1", Type: "A", Value: "192.168.1.2", TTL: 600})
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

func TestDeleteRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.DeleteRecord(t.Context(), ddns.RecordInfo{Name: "test.example.com", ID: "192.168.1.1"})
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

func TestGetRecords(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	records, err := client.GetRecords(t.Context(), "test.example.com", "A")
	if err != nil {
		t.Errorf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

func TestGetRootDomain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"domain": "example.com"}`))
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	subDomain, domain, err := client.getRootDomain(t.Context(), "test.example.com")
	if err != nil {
		t.Errorf("getRootDomain failed: %v", err)
	}

	if subDomain != "test" || domain != "example.com" {
		t.Errorf("Expected subDomain=test, domain=example.com, got subDomain=%s, domain=%s", subDomain, domain)
	}
}

func TestMakeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	var result map[string]any
	err := client.makeRequest(t.Context(), "GET", ts.URL, nil, &result)
	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
}
