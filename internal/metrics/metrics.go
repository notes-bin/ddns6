// Package metrics 提供可选的 Prometheus 文本格式指标导出。
//
// 默认不监听端口；仅当配置了非空 listen address 时由 Serve 启动 HTTP 服务。
// 指标基数刻意保持极低，禁止按 domain/IP 打点。
package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// PromQL 参考（scrape 后可用）:
//
//	ddns6_last_success_timestamp > 0
//	time() - ddns6_last_success_timestamp > 1800   # 超过 30 分钟未成功
//	rate(ddns6_sync_total{result="error"}[15m])
//	rate(ddns6_ipv6_fetch_total{result="error"}[15m])

var (
	syncOK      atomic.Uint64
	syncErr     atomic.Uint64
	ipv6OK      atomic.Uint64
	ipv6Err     atomic.Uint64
	lastSuccess atomic.Int64 // Unix 秒；0 表示尚未成功
	syncSkipped atomic.Uint64
)

// IncSync 记录一轮同步结果。
func IncSync(ok bool) {
	if ok {
		syncOK.Add(1)
		return
	}
	syncErr.Add(1)
}

// IncIPv6Fetch 记录一次 IPv6 获取结果。
func IncIPv6Fetch(ok bool) {
	if ok {
		ipv6OK.Add(1)
		return
	}
	ipv6Err.Add(1)
}

// IncSyncSkipped 记录因单飞而跳过的触发次数。
func IncSyncSkipped() {
	syncSkipped.Add(1)
}

// MarkSuccess 更新最近一次成功同步的 Unix 时间戳。
func MarkSuccess() {
	lastSuccess.Store(time.Now().Unix())
}

// Handler 返回 Prometheus 文本格式的 /metrics 处理器。
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(w, "# HELP ddns6_sync_total Total DNS sync cycles by result.\n")
		_, _ = fmt.Fprintf(w, "# TYPE ddns6_sync_total counter\n")
		_, _ = fmt.Fprintf(w, "ddns6_sync_total{result=\"ok\"} %d\n", syncOK.Load())
		_, _ = fmt.Fprintf(w, "ddns6_sync_total{result=\"error\"} %d\n", syncErr.Load())

		_, _ = fmt.Fprintf(w, "# HELP ddns6_ipv6_fetch_total Total IPv6 address fetch attempts by result.\n")
		_, _ = fmt.Fprintf(w, "# TYPE ddns6_ipv6_fetch_total counter\n")
		_, _ = fmt.Fprintf(w, "ddns6_ipv6_fetch_total{result=\"ok\"} %d\n", ipv6OK.Load())
		_, _ = fmt.Fprintf(w, "ddns6_ipv6_fetch_total{result=\"error\"} %d\n", ipv6Err.Load())

		_, _ = fmt.Fprintf(w, "# HELP ddns6_sync_skipped_total Triggers skipped because a sync was already in progress.\n")
		_, _ = fmt.Fprintf(w, "# TYPE ddns6_sync_skipped_total counter\n")
		_, _ = fmt.Fprintf(w, "ddns6_sync_skipped_total %d\n", syncSkipped.Load())

		_, _ = fmt.Fprintf(w, "# HELP ddns6_last_success_timestamp Unix timestamp of last successful sync; 0 if never.\n")
		_, _ = fmt.Fprintf(w, "# TYPE ddns6_last_success_timestamp gauge\n")
		_, _ = fmt.Fprintf(w, "ddns6_last_success_timestamp %d\n", lastSuccess.Load())
	})
}

// validateListenAddr 要求 metrics 仅绑定 loopback / localhost，防止误暴露公网。
func validateListenAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid metrics addr %q: %w", addr, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return fmt.Errorf("metrics addr must bind loopback (got %q); use 127.0.0.1:%s", addr, port)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return fmt.Errorf("metrics addr host must be loopback or localhost (got %q)", host)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("metrics addr must bind loopback (got %q); use 127.0.0.1:%s", addr, port)
	}
	return nil
}

// Serve 在 addr 上提供 /metrics，直到 ctx 取消。addr 为空则立即返回。
//
// 仅允许 loopback / localhost，避免误绑 0.0.0.0 暴露运营指标。
func Serve(ctx context.Context, addr string) error {
	if addr == "" {
		return nil
	}
	if err := validateListenAddr(addr); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", Handler())

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("metrics listen %s: %w", addr, err)
	}

	slog.Info("metrics endpoint listening", "module", "metrics", "addr", ln.Addr().String())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// ResetForTest 仅供测试重置计数器。
func ResetForTest() {
	syncOK.Store(0)
	syncErr.Store(0)
	ipv6OK.Store(0)
	ipv6Err.Store(0)
	lastSuccess.Store(0)
	syncSkipped.Store(0)
}
