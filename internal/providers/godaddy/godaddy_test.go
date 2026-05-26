package godaddy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var ctx = context.Background()

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

func TestAddDomainRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.AddDomainRecord(ctx, "test.example.com", "A", "192.168.1.1", 600)
	if err != nil {
		t.Errorf("AddDomainRecord failed: %v", err)
	}
}

func TestModifyDomainRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.ModifyDomainRecord(ctx, "test.example.com", "A", "192.168.1.1", "192.168.1.2", 600)
	if err != nil {
		t.Errorf("ModifyDomainRecord failed: %v", err)
	}
}

func TestDeleteDomainRecord(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.DeleteDomainRecord(ctx, "test.example.com", "A", "192.168.1.1")
	if err != nil {
		t.Errorf("DeleteDomainRecord failed: %v", err)
	}
}

func TestGetDomainRecords(t *testing.T) {
	ts := newGoDaddyTestServer(t)
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	records, err := client.GetDomainRecords(ctx, "test.example.com", "A")
	if err != nil {
		t.Errorf("GetDomainRecords failed: %v", err)
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

	subDomain, domain, err := client.getRootDomain(ctx, "test.example.com")
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

	var result map[string]interface{}
	err := client.makeRequest(ctx, "GET", ts.URL, nil, &result)
	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
}
