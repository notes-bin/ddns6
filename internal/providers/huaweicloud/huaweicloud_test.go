package huaweicloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var ctx = context.Background()

// newHuaweiTestServer creates a test server that handles IAM, zone lookup, and record operations
func newHuaweiTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/auth/tokens" {
			w.Header().Set("X-Subject-Token", "test-token")
			w.WriteHeader(http.StatusCreated)
			return
		}
		if strings.Contains(r.URL.Path, "/v2/zones") && !strings.Contains(r.URL.Path, "/recordsets") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"zones": [{"id": "zone123", "name": "example.com."}]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusAccepted)
		}
		w.Write([]byte(`{"id": "123456"}`))
	}))
}

func TestAddRecord(t *testing.T) {
	ts := newHuaweiTestServer(t)
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain",
		WithIAMURL(ts.URL),
		WithDNSURL(ts.URL),
	)

	err := client.AddRecord(ctx, "test.example.com", "A", "192.168.1.1", 600)
	if err != nil {
		t.Errorf("AddRecord failed: %v", err)
	}
}

func TestModifyRecord(t *testing.T) {
	ts := newHuaweiTestServer(t)
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain",
		WithIAMURL(ts.URL),
		WithDNSURL(ts.URL),
	)

	err := client.ModifyRecord(ctx, "test.example.com", "123456", "A", "192.168.1.2", 600)
	if err != nil {
		t.Errorf("ModifyRecord failed: %v", err)
	}
}

func TestDeleteRecord(t *testing.T) {
	ts := newHuaweiTestServer(t)
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain",
		WithIAMURL(ts.URL),
		WithDNSURL(ts.URL),
	)

	err := client.DeleteRecord(ctx, "test.example.com", "123456")
	if err != nil {
		t.Errorf("DeleteRecord failed: %v", err)
	}
}

func TestGetRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/auth/tokens" {
			w.Header().Set("X-Subject-Token", "test-token")
			w.WriteHeader(http.StatusCreated)
			return
		}
		if strings.Contains(r.URL.Path, "/v2/zones") && !strings.Contains(r.URL.Path, "/recordsets") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"zones": [{"id": "zone123", "name": "example.com."}]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"recordsets": [{"id": "123456", "name": "test.example.com.", "type": "A", "records": ["192.168.1.1"], "ttl": 600}]}`))
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain",
		WithIAMURL(ts.URL),
		WithDNSURL(ts.URL),
	)

	records, err := client.GetRecords(ctx, "test.example.com", "")
	if err != nil {
		t.Errorf("GetRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

func TestGetDomainRecord(t *testing.T) {
	ts := newHuaweiTestServer(t)
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain",
		WithIAMURL(ts.URL),
		WithDNSURL(ts.URL),
	)

	record, err := client.GetDomainRecord(ctx, "test.example.com", "123456")
	if err != nil {
		t.Fatalf("GetDomainRecord failed: %v", err)
	}

	if record.ID != "123456" {
		t.Errorf("Expected record ID 123456, got %s", record.ID)
	}
}

func TestGetToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Subject-Token", "test-token")
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithIAMURL(ts.URL))

	token, err := client.getToken(ctx)
	if err != nil {
		t.Errorf("getToken failed: %v", err)
	}

	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", token)
	}
}

func TestGetZoneID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"zones": [{"id": "zone123", "name": "example.com."}]}`))
	}))
	defer ts.Close()

	client := NewClient("testUser", "testPass", "testDomain", WithDNSURL(ts.URL))

	zoneID, err := client.getZoneID(ctx, "test-token", "test.example.com")
	if err != nil {
		t.Errorf("getZoneID failed: %v", err)
	}

	if zoneID != "zone123" {
		t.Errorf("Expected zone ID 'zone123', got '%s'", zoneID)
	}
}
