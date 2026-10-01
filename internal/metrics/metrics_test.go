package metrics

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHandler_ExposesCounters 验证 /metrics 输出含同步、IPv6、跳过与成功时间戳指标。
func TestHandler_ExposesCounters(t *testing.T) {
	ResetForTest()
	IncIPv6Fetch(true)
	IncSync(true)
	MarkSuccess()
	IncSyncSkipped()

	rr := httptest.NewRecorder()
	Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	s := string(body)
	for _, want := range []string{
		`ddns6_sync_total{result="ok"} 1`,
		`ddns6_ipv6_fetch_total{result="ok"} 1`,
		`ddns6_sync_skipped_total 1`,
		`ddns6_last_success_timestamp `,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("metrics 缺少 %q\n%s", want, s)
		}
	}
}

// TestServe_EmptyAddr 验证空地址立即返回且不监听。
func TestServe_EmptyAddr(t *testing.T) {
	if err := Serve(t.Context(), ""); err != nil {
		t.Fatalf("空 addr 应返回 nil: %v", err)
	}
}

// TestServe_RejectsNonLoopback 验证非 loopback 地址被拒绝（仅允许环回绑定）。
func TestServe_RejectsNonLoopback(t *testing.T) {
	err := Serve(t.Context(), "0.0.0.0:0")
	if err == nil {
		t.Fatal("非 loopback 应拒绝")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("错误应提示 loopback: %v", err)
	}
}

// TestServe_ListenAndShutdown 验证 loopback 上监听 /metrics 后 context 取消可优雅退出。
func TestServe_ListenAndShutdown(t *testing.T) {
	ResetForTest()
	IncSync(false)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("预留端口失败: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, addr)
	}()

	var resp *http.Response
	deadline := time.After(2 * time.Second)
	for {
		resp, err = http.Get("http://" + addr + "/metrics")
		if err == nil {
			break
		}
		select {
		case <-deadline:
			cancel()
			t.Fatalf("metrics 端点未就绪: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	if !strings.Contains(string(body), `ddns6_sync_total{result="error"} 1`) {
		t.Fatalf("Serve 暴露的指标不符合预期:\n%s", body)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("缺少 nosniff header: %q", got)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve 退出应返回 nil: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve 未在取消后退出")
	}
}
