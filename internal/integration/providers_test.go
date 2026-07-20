//go:build integration

// Package integration 包含 ddns6 的集成测试套件。
//
// 运行方式：
//
//	go test -tags=integration -v ./internal/integration/...
//
// 这些测试使用模拟 HTTP 服务器验证完整的 API 交互流程，
// 包括签名算法正确性、错误处理和边缘场景。
package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/internal/providers/alicloud"
	"github.com/notes-bin/ddns6/internal/providers/baiducloud"
	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
	"github.com/notes-bin/ddns6/internal/providers/digitalocean"
	"github.com/notes-bin/ddns6/internal/providers/dnspod"
	"github.com/notes-bin/ddns6/internal/providers/godaddy"
	"github.com/notes-bin/ddns6/internal/providers/huaweicloud"
	"github.com/notes-bin/ddns6/internal/providers/porkbun"
	"github.com/notes-bin/ddns6/internal/providers/tencent"
)

// ============================================================
// 签名算法正确性测试
//
// 验证各服务商的签名/认证头格式是否正确生成，
// 确保 mock server 接收到合法请求。
// ============================================================

// TestTencentCloudSigning 验证腾讯云 TC3-HMAC-SHA256 签名头格式
func TestTencentCloudSigning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		if r.Header.Get("X-TC-Timestamp") == "" {
			t.Fatal("missing X-TC-Timestamp header")
		}
		if r.Header.Get("X-TC-Action") == "" {
			t.Fatal("missing X-TC-Action header")
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "TC3-HMAC-SHA256") {
			t.Fatalf("Authorization should start with TC3-HMAC-SHA256, got: %s", auth)
		}
		if !strings.Contains(auth, "SignedHeaders=content-type;host;x-tc-action") {
			t.Fatalf("Authorization missing SignedHeaders, got: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response":{"DomainList":[{"Domain":"example.com"}],"RequestId":"test"}}`))
	}))
	defer server.Close()

	client := tencent.NewDNSPod("test-secret-id", "test-secret-key",
		tencent.WithAPIUrl(server.URL))

	_, err := client.GetRecords(context.Background(), "example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
}

// TestHuaweiCloudSigning 验证华为云 SDK-HMAC-SHA256 签名头格式
func TestHuaweiCloudSigning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		if r.Header.Get("X-Sdk-Date") == "" {
			t.Fatal("missing X-Sdk-Date header")
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "SDK-HMAC-SHA256") {
			t.Fatalf("Authorization should start with SDK-HMAC-SHA256, got: %s", auth)
		}
		if !strings.Contains(auth, "SignedHeaders=") {
			t.Fatalf("Authorization missing SignedHeaders, got: %s", auth)
		}
		if !strings.Contains(auth, "Signature=") {
			t.Fatalf("Authorization missing Signature, got: %s", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/v2/zones") && !strings.Contains(r.URL.Path, "/recordsets") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"zones": [{"id": "zone123", "name": "example.com."}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"recordsets": []}`))
	}))
	defer server.Close()

	client := huaweicloud.NewClient("test-access-key", "test-secret-key",
		huaweicloud.WithBaseURL(server.URL))

	_, err := client.GetRecords(context.Background(), "example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
}

// TestBaiduCloudSigning 验证百度云 BCE 签名头格式
func TestBaiduCloudSigning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "bce-auth-v1") {
			t.Fatalf("Authorization should start with bce-auth-v1, got: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": [], "totalCount": 0}`))
	}))
	defer server.Close()

	client := baiducloud.NewClient("test-key", "test-secret",
		baiducloud.WithBaseURL(server.URL))

	_, err := client.GetRecords(context.Background(), "example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
}

// TestAliCloudSigning 验证阿里云 HMAC-SHA1 签名
func TestAliCloudSigning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("Signature") == "" {
			t.Fatal("missing Signature parameter")
		}
		if query.Get("AccessKeyId") == "" {
			t.Fatal("missing AccessKeyId parameter")
		}
		if query.Get("SignatureMethod") != "HMAC-SHA1" {
			t.Fatalf("expected HMAC-SHA1, got %s", query.Get("SignatureMethod"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"TotalCount":1,"DomainRecords":{"Record":[{"RecordId":"1","DomainName":"example.com","RR":"@","Type":"AAAA","Value":"2001:db8::1","TTL":600}]}}`))
	}))
	defer server.Close()

	client := alicloud.NewClient("test-key", "test-secret",
		alicloud.WithBaseURL(server.URL))

	_, err := client.GetRecords(context.Background(), "example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
}

// TestCloudflareSigning 验证 Cloudflare Bearer Token 认证
func TestCloudflareSigning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("expected Bearer token, got: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true, "result": [{"id": "zone1", "name": "example.com"}]}`))
	}))
	defer server.Close()

	client := cloudflare.NewClient(
		cloudflare.WithAPIToken("test-token"),
		cloudflare.WithBaseURL(server.URL))

	_, err := client.GetRecords(context.Background(), "example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
}

// ============================================================
// Error 处理测试
//
// 验证各 provider 正确处理各种 HTTP 错误响应
// ============================================================

// TestProviderHTTPErrors 验证 Provider 正确处理 HTTP 错误状态码
func TestProviderHTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		shouldFail bool
	}{
		{name: "bad request", statusCode: http.StatusBadRequest, response: `{}`, shouldFail: true},
		{name: "unauthorized", statusCode: http.StatusUnauthorized, response: `{}`, shouldFail: true},
		{name: "not found", statusCode: http.StatusNotFound, response: `{}`, shouldFail: true},
		{name: "rate limited", statusCode: http.StatusTooManyRequests, response: `{}`, shouldFail: true},
		{name: "internal error", statusCode: http.StatusInternalServerError, response: `{}`, shouldFail: true},
		{name: "service unavailable", statusCode: http.StatusServiceUnavailable, response: `{}`, shouldFail: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := cloudflare.NewClient(
				cloudflare.WithAPIToken("test-token"),
				cloudflare.WithBaseURL(server.URL+"/client/v4"))

			err := client.AddRecord(context.Background(), ddns.RecordInfo{Name: "test.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
			if tt.shouldFail && err == nil {
				t.Errorf("expected error for status %d, got nil", tt.statusCode)
			}
			if !tt.shouldFail && err != nil {
				t.Errorf("unexpected error for status %d: %v", tt.statusCode, err)
			}
		})
	}
}

// TestDNSPodAPIError 验证 DNSPod 的业务错误码处理
func TestDNSPodAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":{"code":"401","message":"invalid login token"}}`))
	}))
	defer server.Close()

	client := dnspod.NewClient("test-token", dnspod.WithBaseURL(server.URL))

	err := client.AddRecord(context.Background(), ddns.RecordInfo{Name: "test.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// TestGodaddyAPIError 验证 GoDaddy 的业务错误处理
func TestGodaddyAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`[{"code":"UNAUTHORIZED","message":"Invalid api key or secret"}]`))
	}))
	defer server.Close()

	_, err := godaddy.NewClient("test-key", "test-secret",
		godaddy.WithBaseURL(server.URL)).
		GetRecords(context.Background(), "example.com", "AAAA")
	if err == nil {
		t.Fatal("expected error for invalid credentials, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// ============================================================
// 服务商特定行为测试
//
// 测试各服务商的特定业务逻辑
// ============================================================

// TestCloudflarePagination 验证 Cloudflare 的分页处理
func TestCloudflarePagination(t *testing.T) {
	dnsRecordCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "/zones") && !strings.Contains(r.URL.Path, "/dns_records") {
			w.Write([]byte(`{"success":true,"result":[{"id":"zone1","name":"test.example.com"}]}`))
			return
		}
		dnsRecordCalls++
		page := r.URL.Query().Get("page")
		if page == "1" || page == "" {
			w.Write([]byte(`{"success":true,"result":[{"id":"rec1","name":"test.example.com","type":"AAAA","content":"2001:db8::1","ttl":120}],"result_info":{"page":1,"per_page":100,"total_pages":2,"total_count":101}}`))
		} else {
			w.Write([]byte(`{"success":true,"result":[{"id":"rec2","name":"test.example.com","type":"AAAA","content":"2001:db8::2","ttl":120}],"result_info":{"page":2,"per_page":100,"total_pages":2,"total_count":101}}`))
		}
	}))
	defer server.Close()

	client := cloudflare.NewClient(
		cloudflare.WithAPIToken("test-token"),
		cloudflare.WithBaseURL(server.URL+"/client/v4"))

	records, err := client.GetRecords(context.Background(), "test.example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records across 2 pages, got %d", len(records))
	}
	if dnsRecordCalls != 2 {
		t.Errorf("expected 2 DNS record page requests, got %d", dnsRecordCalls)
	}
}

// TestMultipleRecordsSameSubdomain 验证同一子域名下多条记录的查询
func TestMultipleRecordsSameSubdomain(t *testing.T) {
	resp := `{"result":[{"recordId":"rec1","domain":"www","rdtype":"AAAA","rdata":"2001:db8::1","ttl":300,"zoneName":"example.com"},{"recordId":"rec2","domain":"www","rdtype":"AAAA","rdata":"2001:db8::2","ttl":300,"zoneName":"example.com"}],"totalCount":2}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := baiducloud.NewClient("test-key", "test-secret",
		baiducloud.WithBaseURL(server.URL))

	records, err := client.GetRecords(context.Background(), "www.example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}

// TestPorkbunFullFlow 验证 Porkbun 完整的 CRUD 交互
func TestPorkbunFullFlow(t *testing.T) {
	var created bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var req struct {
			APIKey       string `json:"apikey"`
			SecretAPIKey string `json:"secretapikey"`
		}
		json.Unmarshal(body, &req)

		if req.APIKey == "" || req.SecretAPIKey == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"status":"ERROR"}`))
			return
		}

		switch {
		case strings.Contains(r.URL.Path, "/retrieveByNameType/"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"SUCCESS","records":[{"id":"rec1","name":"test.example.com","type":"AAAA","content":"2001:db8::1","ttl":"600"}]}`))

		case strings.Contains(r.URL.Path, "/create/"):
			created = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"SUCCESS","id":"new-record"}`))

		case strings.Contains(r.URL.Path, "/editByNameType/"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"SUCCESS"}`))

		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"status":"ERROR"}`))
		}
	}))
	defer server.Close()

	client := porkbun.NewClient("test-api-key", "test-secret-api-key",
		porkbun.WithBaseURL(server.URL))

	records, err := client.GetRecords(context.Background(), "test.example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}

	err = client.ModifyRecord(context.Background(), ddns.RecordInfo{Name: "test.example.com", ID: "rec1", Type: "AAAA", Value: "2001:db8::2", TTL: 600})
	if err != nil {
		t.Fatalf("ModifyRecord failed: %v", err)
	}
	if created {
		t.Error("ModifyRecord should not create a new record")
	}
}

// TestDigitalOceanFullFlow 验证 DigitalOcean 完整的 CRUD 交互
func TestDigitalOceanFullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Unauthorized"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch {
		case strings.Contains(r.URL.Path, "/domains/example.com/records"):
			if r.Method == http.MethodGet {
				w.Write([]byte(`{"domain_records":[{"id":42,"name":"www","type":"AAAA","data":"2001:db8::1","ttl":600}]}`))
			} else if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"domain_record":{"id":99}}`))
			} else if r.Method == http.MethodPut {
				w.Write([]byte(`{"domain_record":{"id":42}}`))
			} else if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := digitalocean.NewClient("test-token",
		digitalocean.WithBaseURL(server.URL))

	records, err := client.GetRecords(context.Background(), "www.example.com", "AAAA")
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}
	if len(records) != 1 || records[0].Value != "2001:db8::1" {
		t.Errorf("unexpected records: %+v", records)
	}

	err = client.ModifyRecord(context.Background(), ddns.RecordInfo{Name: "www.example.com", ID: "42", Type: "AAAA", Value: "2001:db8::2", TTL: 600})
	if err != nil {
		t.Fatalf("ModifyRecord failed: %v", err)
	}

	err = client.DeleteRecord(context.Background(), ddns.RecordInfo{Name: "www.example.com", ID: "42"})
	if err != nil {
		t.Fatalf("DeleteRecord failed: %v", err)
	}
}

// ============================================================
// 超时与取消测试
//
// 验证 provider 正确处理 context 超时和取消
// ============================================================

// TestContextTimeout 验证 context 超时后快速返回错误
func TestContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := cloudflare.NewClient(
		cloudflare.WithAPIToken("test-token"),
		cloudflare.WithBaseURL(server.URL+"/client/v4"),
		cloudflare.WithHTTPClient(&http.Client{Timeout: 1 * time.Second}))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.AddRecord(ctx, ddns.RecordInfo{Name: "test.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	t.Logf("got expected timeout error: %v", err)
}

// TestContextCancel 验证 context 取消后快速返回错误
func TestContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := cloudflare.NewClient(
		cloudflare.WithAPIToken("test-token"),
		cloudflare.WithBaseURL(server.URL+"/client/v4"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.AddRecord(ctx, ddns.RecordInfo{Name: "test.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	t.Logf("got expected cancellation error: %v", err)
}
