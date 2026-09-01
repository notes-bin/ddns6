package namecheap

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

const (
	testGetHostsOK = `<?xml version="1.0"?><ApiResponse Status="OK"><CommandResponse><DomainDNSGetHostsResult Domain="example.com"><host HostId="1" Name="www" Type="AAAA" Address="2001:db8::1" MXPref="10" TTL="600"/></DomainDNSGetHostsResult></CommandResponse></ApiResponse>`
	testSetHostsOK = `<?xml version="1.0"?><ApiResponse Status="OK"><CommandResponse/></ApiResponse>`
)

// newTestClient 创建指向 mock 服务器的 Namecheap 客户端。
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewClient("key", "user", "127.0.0.1", WithBaseURL(server.URL))
}

// defaultHandler 为 getHosts/setHosts 提供默认成功响应。
func defaultHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.Contains(r.URL.RawQuery, "getHosts"):
		fmt.Fprint(w, testGetHostsOK)
	case strings.Contains(r.URL.RawQuery, "setHosts"):
		fmt.Fprint(w, testSetHostsOK)
	default:
		http.NotFound(w, r)
	}
}

// TestClient_GetRecords 验证 getHosts 解析与记录映射。
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

// TestClient_AddRecord 验证添加记录会调用 setHosts。
func TestClient_AddRecord(t *testing.T) {
	var setHostsCalled bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "setHosts") {
			setHostsCalled = true
			fmt.Fprint(w, testSetHostsOK)
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
	if !setHostsCalled {
		t.Error("expected setHosts to be called")
	}
}

// TestClient_ModifyRecord 验证修改记录会更新 setHosts 中的地址。
func TestClient_ModifyRecord(t *testing.T) {
	var newAddress string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "setHosts") {
			newAddress = r.URL.Query().Get("Address1")
			fmt.Fprint(w, testSetHostsOK)
			return
		}
		defaultHandler(w, r)
	})
	err := client.ModifyRecord(t.Context(), ddns.RecordInfo{
		Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::3", TTL: 600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newAddress != "2001:db8::3" {
		t.Errorf("expected updated address 2001:db8::3, got %q", newAddress)
	}
}

// TestClient_DeleteRecord 验证删除后 setHosts 提交空主机列表。
func TestClient_DeleteRecord(t *testing.T) {
	var hostCount int
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "setHosts") {
			for i := 1; r.URL.Query().Get("HostName"+fmt.Sprint(i)) != ""; i++ {
				hostCount++
			}
			fmt.Fprint(w, testSetHostsOK)
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
	if hostCount != 0 {
		t.Errorf("expected 0 hosts after delete, got %d", hostCount)
	}
}

// TestClient_ApiError 验证 XML 错误响应被正确返回。
func TestClient_ApiError(t *testing.T) {
	const errXML = `<?xml version="1.0"?><ApiResponse Status="ERROR"><Errors><Error>Invalid API Key</Error></Errors></ApiResponse>`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, errXML)
	})
	_, err := client.GetRecords(t.Context(), "www.example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid API Key") {
		t.Errorf("unexpected error: %v", err)
	}
}
