package ipaddr_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// TestDnsFetcher 测试 DnsFetcher 的功能
func TestDnsFetcher(t *testing.T) {
	dnsServer := "2001:4860:4860::8888" // Google DNS
	fetcher := ipaddr.NewDnsFetcher(dnsServer)

	if fetcher.String() != dnsServer {
		t.Errorf("Expected DnsFetcher string to be %s, got %s", dnsServer, fetcher.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 尝试获取 IPv6 地址
	ip, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Logf("DnsFetcher failed (possibly no IPv6 network): %v", err)
	} else {
		// 验证返回的是 IPv6 地址
		if ip.To4() != nil {
			t.Error("Expected IPv6 address, got IPv4 address")
		}
		if ip.To16() == nil {
			t.Error("Expected valid IP address, got nil")
		}
	}
}

// TestHttpIPv6Fetcher 测试 HttpIPv6Fetcher 的功能
func TestHttpIPv6Fetcher(t *testing.T) {
	// 创建测试 HTTP 服务器
	mockIPv6 := "2001:db8::1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockIPv6))
	}))
	defer server.Close()

	// 创建 HttpIPv6Fetcher
	fetcher := ipaddr.NewHttpIPv6Fetcher(server.URL)

	if fetcher.String() != server.URL {
		t.Errorf("Expected HttpIPv6Fetcher string to be %s, got %s", server.URL, fetcher.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试正常情况
	ip, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Errorf("Expected HttpIPv6Fetcher to succeed, got error: %v", err)
	}

	if ip.String() != mockIPv6 {
		t.Errorf("Expected IPv6 address %s, got %s", mockIPv6, ip.String())
	}

	// 测试错误情况 - 无效 IP 地址
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid-ip"))
	}))
	defer errorServer.Close()

	errorFetcher := ipaddr.NewHttpIPv6Fetcher(errorServer.URL)
	_, err = errorFetcher.Fetch(ctx)
	if err == nil {
		t.Error("Expected HttpIPv6Fetcher to fail with invalid IP, got success")
	}
}
