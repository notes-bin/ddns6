package ddns

import (
	"context"
	"errors"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// stubFetcher 固定返回预设 IP 或错误，供 RunService / IPv6Addr 测试。
type stubFetcher struct {
	ip  net.IP
	err error
}

func (s *stubFetcher) Fetch(context.Context) (net.IP, error) {
	return s.ip, s.err
}

// TestDefaultIPv6Fetchers 验证默认获取器列表非空且每次调用返回独立切片。
func TestDefaultIPv6Fetchers(t *testing.T) {
	a := DefaultIPv6Fetchers()
	b := DefaultIPv6Fetchers()
	if len(a) == 0 {
		t.Fatal("DefaultIPv6Fetchers 不应返回空列表")
	}
	if len(a) != len(b) {
		t.Fatalf("两次调用长度不一致: %d vs %d", len(a), len(b))
	}
	orig := b[0]
	a[0] = &stubFetcher{}
	if b[0] != orig {
		t.Error("修改一次返回值不应影响另一次调用的切片内容")
	}
}

// TestPlatformTriggerMode 验证非 Linux 平台触发模式描述。
func TestPlatformTriggerMode(t *testing.T) {
	if got := platformTriggerMode(); got == "" {
		t.Error("platformTriggerMode 不应返回空串")
	}
}

// TestPollingLoop_TriggersThenStops 验证轮询发送触发信号，取消后退出。
func TestPollingLoop_TriggersThenStops(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	triggerCh := make(chan struct{}, 1)

	done := make(chan struct{})
	go func() {
		pollingLoop(ctx, triggerCh, 20*time.Millisecond)
		close(done)
	}()

	select {
	case <-triggerCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("轮询未在预期时间内触发")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("取消后 pollingLoop 未退出")
	}
}

// TestPollingLoop_NonBlockingWhenFull 验证通道已满时跳过发送且不阻塞。
func TestPollingLoop_NonBlockingWhenFull(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	triggerCh := make(chan struct{}, 1)
	triggerCh <- struct{}{} // 预先填满

	const interval = 10 * time.Millisecond
	done := make(chan struct{})
	go func() {
		pollingLoop(ctx, triggerCh, interval)
		close(done)
	}()

	// 等待若干次 ticker，确认满通道路径不阻塞；再取消
	select {
	case <-time.After(5 * interval):
	case <-done:
		t.Fatal("通道已满时 pollingLoop 不应提前退出")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("通道已满时 pollingLoop 应能在取消后退出")
	}
}

// TestStartTrigger_Polling 验证非 Linux 平台 startTrigger 按 interval 轮询触发。
//
// Linux 走 Netlink（需真实地址事件 + 10s 防抖），不会按 interval 定期触发；
// 轮询逻辑由 TestPollingLoop_* 覆盖，取消行为见 TestStartTrigger_Cancel。
func TestStartTrigger_Polling(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux 使用 netlink 事件触发，不按 interval 轮询")
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch := startTrigger(ctx, 15*time.Millisecond, "")

	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("startTrigger 未触发")
	}
}

// TestStartTrigger_Cancel 验证 startTrigger 在 context 取消后不再投递触发。
func TestStartTrigger_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	ch := startTrigger(ctx, 15*time.Millisecond, "")
	cancel()

	// 排空可能已缓冲的一次信号，之后不应再有新触发
	select {
	case <-ch:
	default:
	}
	select {
	case <-ch:
		t.Fatal("取消后不应再收到触发信号")
	case <-time.After(80 * time.Millisecond):
	}
}

// TestSyncAllDomains_FailFast 验证 failFast 时首个错误立即返回。
func TestSyncAllDomains_FailFast(t *testing.T) {
	domains := []*Domain{
		{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600},
	}
	m := &mockProvider{getErr: errors.New("api down")}
	err := syncAllDomains(t.Context(), domains, net.ParseIP("2001:db8::1"), m, true)
	if err == nil {
		t.Fatal("failFast 时期望返回错误")
	}
}

// TestSyncAllDomains_ContinueOnError 验证非 failFast 时各组仍跑完，并返回汇总错误。
func TestSyncAllDomains_ContinueOnError(t *testing.T) {
	domains := []*Domain{
		{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600},
		{Domain: "example.com", SubDomain: "api", Type: "AAAA", TTL: 600},
	}
	m := &mockProvider{getErr: errors.New("api down")}
	err := syncAllDomains(t.Context(), domains, net.ParseIP("2001:db8::1"), m, false)
	if err == nil {
		t.Fatal("非 failFast 但存在失败组时应返回汇总错误")
	}
}

// TestSyncAllDomains_Success 验证全部域名同步成功。
func TestSyncAllDomains_Success(t *testing.T) {
	addr := net.ParseIP("2001:db8::1")
	domains := []*Domain{
		{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600},
	}
	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
	}
	if err := syncAllDomains(t.Context(), domains, addr, m, true); err != nil {
		t.Fatalf("syncAllDomains 成功路径不应失败: %v", err)
	}
}

// TestSyncAllDomains_MergesGetRecords 验证同根域名多子域只调用一次 GetRecords（root-domain merge）。
func TestSyncAllDomains_MergesGetRecords(t *testing.T) {
	addr := net.ParseIP("2001:db8::2")
	domains := []*Domain{
		{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600},
		{Domain: "example.com", SubDomain: "api", Type: "AAAA", TTL: 600},
	}
	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
			{ID: "2", Name: "api.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
	}
	if err := syncAllDomains(t.Context(), domains, addr, m, true); err != nil {
		t.Fatalf("syncAllDomains 不应失败: %v", err)
	}
	if m.getCalls != 1 {
		t.Fatalf("同 zone 多子域应只 GetRecords 一次, got %d", m.getCalls)
	}
}

// TestRunService_InitialFetchFailed 验证首次获取 IPv6 失败时立即返回。
func TestRunService_InitialFetchFailed(t *testing.T) {
	domains := []*Domain{{Domain: "example.com", SubDomain: "@", Type: "AAAA", TTL: 600}}
	err := RunService(domains, &mockProvider{}, time.Minute, []ipaddr.IPv6Fetcher{
		&stubFetcher{err: errors.New("no ipv6")},
	}, "", "")
	if err == nil {
		t.Fatal("初始 IPv6 获取失败时应返回错误")
	}
}

// TestRunService_InitialSyncFailed 验证首次同步失败时立即返回。
func TestRunService_InitialSyncFailed(t *testing.T) {
	domains := []*Domain{{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}}
	err := RunService(domains, &mockProvider{getErr: errors.New("sync fail")}, time.Minute, []ipaddr.IPv6Fetcher{
		&stubFetcher{ip: net.ParseIP("2001:db8::1")},
	}, "", "")
	if err == nil {
		t.Fatal("初始同步失败时应返回错误")
	}
}

// countingFetcher 首次成功、后续失败，用于覆盖 RunService 触发后的获取失败分支。
type countingFetcher struct {
	n           int
	ip          net.IP
	mu          sync.Mutex
	onSecondTry chan struct{} // 第二次 Fetch 时关闭（至多一次）
}

func (c *countingFetcher) Fetch(context.Context) (net.IP, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	if c.n == 1 {
		return c.ip, nil
	}
	if c.n == 2 && c.onSecondTry != nil {
		select {
		case <-c.onSecondTry:
		default:
			close(c.onSecondTry)
		}
	}
	return nil, errors.New("trigger fetch failed")
}

// TestRunService_GracefulShutdown 验证服务启动、轮询触发获取失败后取消 context 可退出。
//
// 无进行中同步时取消应立即退出；有同步时最多等待约 5 秒。
// 通过可取消 context 注入关机，避免向 go test 进程发送真实 SIGTERM。
func TestRunService_GracefulShutdown(t *testing.T) {
	domains := []*Domain{{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}}
	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
	}
	secondTry := make(chan struct{})
	fetcher := &countingFetcher{ip: net.ParseIP("2001:db8::1"), onSecondTry: secondTry}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runService(ctx, domains, m, 30*time.Millisecond, []ipaddr.IPv6Fetcher{fetcher}, "", "")
	}()

	// 等待至少一次轮询触发（覆盖 trigger 上获取失败分支）后再取消
	select {
	case <-secondTry:
	case <-time.After(2 * time.Second):
		t.Fatal("未等到第二次 IPv6 获取（轮询触发）")
	}
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("优雅退出应返回 nil: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runService 未在超时前退出")
	}
}
