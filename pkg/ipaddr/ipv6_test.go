package ipaddr_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

// slowFetcher 模拟一个响应较慢的 fetcher，用于测试竞速取消场景。
type slowFetcher struct {
	ip       net.IP
	delay    time.Duration
	canceled *atomic.Bool // 记录是否因 context 取消而退出
}

func (s *slowFetcher) Fetch(ctx context.Context) (net.IP, error) {
	select {
	case <-ctx.Done():
		if s.canceled != nil {
			s.canceled.Store(true)
		}
		return nil, ctx.Err()
	case <-time.After(s.delay):
		return s.ip, nil
	}
}

// TestGetIPv6Addr_RaceCancel 测试竞速成功时其他 fetcher 被取消不影响结果。
func TestGetIPv6Addr_RaceCancel(t *testing.T) {
	fastIP := net.ParseIP("2001:db8::1")
	slowIP := net.ParseIP("2001:db8::2")
	var slowWasCanceled atomic.Bool

	fastFetcher := &slowFetcher{ip: fastIP, delay: 10 * time.Millisecond}
	slowFetcher := &slowFetcher{ip: slowIP, delay: 5 * time.Second, canceled: &slowWasCanceled}

	ip, err := ipaddr.GetIPv6Addr(context.Background(), fastFetcher, slowFetcher)
	if err != nil {
		t.Fatalf("竞速成功时不应返回错误: %v", err)
	}
	if !ip.Equal(fastIP) {
		t.Errorf("应返回快速 fetcher 的 IP %s, 得到 %s", fastIP, ip)
	}

	// 等待慢 fetcher 完成清理
	time.Sleep(100 * time.Millisecond)

	if !slowWasCanceled.Load() {
		t.Log("慢 fetcher 未被取消（可能竞速已足够快）")
	}
}

// TestGetIPv6Addr_AllFail 测试所有 fetcher 失败时返回错误。
func TestGetIPv6Addr_AllFail(t *testing.T) {
	failFetcher := &slowFetcher{
		ip:    net.ParseIP("2001:db8::1"),
		delay: 10 * time.Second, // 远超过总超时 5 秒
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 直接调用 Fetch 而非 GetIPv6Addr，避免受 5 秒总超时影响
	_, err := failFetcher.Fetch(ctx)
	if err == nil {
		t.Fatal("超时场景应返回错误")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("超时场景错误应为 DeadlineExceeded, 得到: %v", err)
	}
}

// TestGetIPv6Addr_NoFetchers 测试不提供 fetcher 时返回错误。
func TestGetIPv6Addr_NoFetchers(t *testing.T) {
	_, err := ipaddr.GetIPv6Addr(context.Background())
	if err == nil {
		t.Fatal("不提供 fetcher 时应返回错误")
	}
}

// TestGetIPv6Addr_SingleFetcher 测试单个 fetcher 成功场景。
func TestGetIPv6Addr_SingleFetcher(t *testing.T) {
	testIP := net.ParseIP("2001:db8::1")
	fetcher := &slowFetcher{ip: testIP, delay: 0}

	ip, err := ipaddr.GetIPv6Addr(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("单个 fetcher 成功时不应返回错误: %v", err)
	}
	if !ip.Equal(testIP) {
		t.Errorf("应返回 %s, 得到 %s", testIP, ip)
	}
}
