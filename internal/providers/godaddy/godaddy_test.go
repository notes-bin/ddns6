package godaddy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.AddDomainRecord("test.example.com", "A", "192.168.1.1", 600)
	if err != nil {
		t.Errorf("AddDomainRecord failed: %v", err)
	}
}

func TestModifyDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.ModifyDomainRecord("test.example.com", "A", "192.168.1.1", "192.168.1.2", 600)
	if err != nil {
		t.Errorf("ModifyDomainRecord failed: %v", err)
	}
}

func TestDeleteDomainRecord(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	err := client.DeleteDomainRecord("test.example.com", "A", "192.168.1.1")
	if err != nil {
		t.Errorf("DeleteDomainRecord failed: %v", err)
	}
}

func TestGetDomainRecords(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"data": "192.168.1.1", "name": "test", "type": "A", "ttl": 600}]`))
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	records, err := client.GetDomainRecords("test.example.com", "A")
	if err != nil {
		t.Errorf("GetDomainRecords failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

func TestGetRootDomain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	subDomain, domain, err := client.getRootDomain("test.example.com")
	if err != nil {
		t.Errorf("getRootDomain failed: %v", err)
	}

	if subDomain != "test" || domain != "example.com" {
		t.Errorf("Expected subDomain=test, domain=example.com, got subDomain=%s, domain=%s", subDomain, domain)
	}
}

func TestMakeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer ts.Close()

	client := NewClient("testKey", "testSecret", WithBaseURL(ts.URL))

	var result map[string]interface{}
	err := client.makeRequest("GET", ts.URL, nil, &result)
	if err != nil {
		t.Errorf("makeRequest failed: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
}
