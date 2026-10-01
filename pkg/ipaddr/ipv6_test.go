package ipaddr_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// TestDNSFetcher 验证 DNSFetcher 的 String 与 Fetch（无 IPv6 时仅记录日志）。
func TestDNSFetcher(t *testing.T) {
	dnsServer := "2001:4860:4860::8888" // Google DNS
	fetcher := ipaddr.NewDNSFetcher(dnsServer)

	if fetcher.String() != dnsServer {
		t.Errorf("Expected DNSFetcher string to be %s, got %s", dnsServer, fetcher.String())
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ip, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Logf("DNSFetcher failed (possibly no IPv6 network): %v", err)
	} else {
		if ip.To4() != nil {
			t.Error("Expected IPv6 address, got IPv4 address")
		}
		if ip.To16() == nil {
			t.Error("Expected valid IP address, got nil")
		}
	}
}

// TestHTTPIPv6Fetcher 用 httptest 验证合法 IPv6 解析与非法正文失败。
func TestHTTPIPv6Fetcher(t *testing.T) {
	mockIPv6 := "2001:db8::1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockIPv6))
	}))
	defer server.Close()

	fetcher := ipaddr.NewHTTPIPv6Fetcher(server.URL)

	if fetcher.String() != server.URL {
		t.Errorf("Expected HTTPIPv6Fetcher string to be %s, got %s", server.URL, fetcher.String())
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ip, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Errorf("Expected HTTPIPv6Fetcher to succeed, got error: %v", err)
	}

	if ip.String() != mockIPv6 {
		t.Errorf("Expected IPv6 address %s, got %s", mockIPv6, ip.String())
	}

	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid-ip"))
	}))
	defer errorServer.Close()

	errorFetcher := ipaddr.NewHTTPIPv6Fetcher(errorServer.URL)
	_, err = errorFetcher.Fetch(ctx)
	if err == nil {
		t.Error("Expected HTTPIPv6Fetcher to fail with invalid IP, got success")
	}
}

// slowFetcher 模拟可配置延迟的 IPv6Fetcher，用于竞速与取消场景。
type slowFetcher struct {
	ip         net.IP
	delay      time.Duration
	canceledCh chan struct{} // 取消时关闭（可选，至多一次）
}

func (s *slowFetcher) Fetch(ctx context.Context) (net.IP, error) {
	select {
	case <-ctx.Done():
		if s.canceledCh != nil {
			select {
			case <-s.canceledCh:
			default:
				close(s.canceledCh)
			}
		}
		return nil, ctx.Err()
	case <-time.After(s.delay):
		return s.ip, nil
	}
}

// TestIPv6Addr_RaceCancel 验证快 fetcher 胜出时慢方取消不影响结果。
func TestIPv6Addr_RaceCancel(t *testing.T) {
	fastIP := net.ParseIP("2001:db8::1")
	slowIP := net.ParseIP("2001:db8::2")
	canceledCh := make(chan struct{})

	fastFetcher := &slowFetcher{ip: fastIP, delay: 10 * time.Millisecond}
	slowFetcher := &slowFetcher{
		ip: slowIP, delay: 5 * time.Second, canceledCh: canceledCh,
	}

	ip, err := ipaddr.IPv6Addr(t.Context(), fastFetcher, slowFetcher)
	if err != nil {
		t.Fatalf("竞速成功时不应返回错误: %v", err)
	}
	if !ip.Equal(fastIP) {
		t.Errorf("应返回快速 fetcher 的 IP %s, 得到 %s", fastIP, ip)
	}

	select {
	case <-canceledCh:
	case <-time.After(2 * time.Second):
		t.Log("慢 fetcher 未被取消（可能竞速已足够快）")
	}
}

// failFetcher 始终返回错误的 IPv6Fetcher。
type failFetcher struct{ err error }

func (f *failFetcher) Fetch(context.Context) (net.IP, error) {
	return nil, f.err
}

// TestIPv6Addr_AllFail 验证全部 fetcher 失败时返回汇总错误。
func TestIPv6Addr_AllFail(t *testing.T) {
	f1 := &failFetcher{err: errors.New("upstream down")}
	f2 := &failFetcher{err: errors.New("dns fail")}

	_, err := ipaddr.IPv6Addr(t.Context(), f1, f2)
	if err == nil {
		t.Fatal("全部失败时应返回错误")
	}
	if !strings.Contains(err.Error(), "all 2 fetchers failed") {
		t.Errorf("错误应包含失败汇总: %v", err)
	}
}

// TestIPv6Addr_NoFetchers 验证未提供 fetcher 时返回错误。
func TestIPv6Addr_NoFetchers(t *testing.T) {
	_, err := ipaddr.IPv6Addr(t.Context())
	if err == nil {
		t.Fatal("不提供 fetcher 时应返回错误")
	}
}

// hungFetcher 忽略 ctx，阻塞到 stop 关闭；started 在进入阻塞前关闭（至多一次）。
type hungFetcher struct {
	stop    chan struct{}
	started chan struct{}
}

func (h *hungFetcher) Fetch(context.Context) (net.IP, error) {
	if h.started != nil {
		select {
		case <-h.started:
		default:
			close(h.started)
		}
	}
	<-h.stop
	return nil, errors.New("stopped")
}

// TestIPv6Addr_ParentCancel 验证忽略 ctx 的 fetcher 下，父取消仍立即返回。
func TestIPv6Addr_ParentCancel(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	started := make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := ipaddr.IPv6Addr(ctx, &hungFetcher{stop: stop, started: started})
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("fetcher 未启动")
	}
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("父 context 取消时应返回 Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("取消后应立即返回")
	}
}

// TestIPv6Addr_SingleFetcher 验证单个 fetcher 成功路径。
func TestIPv6Addr_SingleFetcher(t *testing.T) {
	testIP := net.ParseIP("2001:db8::1")
	fetcher := &slowFetcher{ip: testIP, delay: 0}

	ip, err := ipaddr.IPv6Addr(t.Context(), fetcher)
	if err != nil {
		t.Fatalf("单个 fetcher 成功时不应返回错误: %v", err)
	}
	if !ip.Equal(testIP) {
		t.Errorf("应返回 %s, 得到 %s", testIP, ip)
	}
}
