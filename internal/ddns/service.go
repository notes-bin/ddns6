package ddns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/internal/httputil"
	"github.com/notes-bin/ddns6/internal/metrics"
	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// DefaultIPv6Fetchers 返回默认的 IPv6 地址获取器列表（每次调用返回新切片，避免调用方污染全局状态）。
//
// 每次触发同步时由 IPv6Addr 随机打乱顺序后并发竞速，取第一个成功结果。
// 包含 HTTP 与 DNS 两种来源，互为备份。
func DefaultIPv6Fetchers() []ipaddr.IPv6Fetcher {
	return []ipaddr.IPv6Fetcher{
		ipaddr.NewHTTPIPv6Fetcher("https://6.ipw.cn"),
		ipaddr.NewHTTPIPv6Fetcher("https://ifconfig.co"),
		ipaddr.NewHTTPIPv6Fetcher("https://v6.ident.me"),
		ipaddr.NewDNSFetcher("2402:4e00::"),
		ipaddr.NewDNSFetcher("2400:3200:baba::1"),
		ipaddr.NewDNSFetcher("2001:4860:4860::8888"),
		ipaddr.NewDNSFetcher("2606:4700:4700::1111"),
	}
}

// RunService 启动 DDNS 服务，持续监听 IPv6 地址变化并更新 DNS 记录。
//
// 参数:
//   - domains: 要更新的域名列表（支持同一根域名下多个子域名）
//   - p: DNS 服务商实现
//   - interval: 非 Linux 平台的轮询间隔（Linux 下由 Netlink 事件驱动，此参数仅作回退）
//   - fetchers: IPv6 地址获取器列表，每次触发时随机顺序并发竞速
//   - iface: 指定监听的网络接口（空字符串表示监听所有接口，仅 Linux Netlink 模式有效）
//   - metricsAddr: 可选 Prometheus /metrics 监听地址（空则不启用）
//
// 返回 error 仅在以下情况返回：
//   - 首次启动获取 IPv6 地址失败
//   - 首次同步 DNS 记录失败
//
// 运行时错误（后续 Netlink 或轮询中的失败）仅记录日志，不影响服务运行。
//
// 退出方式：
//   - 收到 SIGINT 或 SIGTERM 后优雅关闭（signal.NotifyContext）
//   - 先取消正在进行的操作，再等待进行中的同步完成（最多 5 秒）
//   - 然后返回 nil
func RunService(domains []*Domain, p DNSProvider, interval time.Duration, fetchers []ipaddr.IPv6Fetcher, iface, metricsAddr string) error {
	slog.Info("starting DDNS update service",
		"module", "ddns",
		"domain_count", len(domains),
		"interval", interval,
		"interface", iface,
		"metrics_addr", metricsAddr)

	// 可取消 context：SIGINT/SIGTERM 时取消并传播到进行中的获取与同步
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runService(ctx, domains, p, interval, fetchers, iface, metricsAddr)
}

// runService 在给定 ctx 下运行主循环；测试可注入可取消 context，避免向进程发真实信号。
func runService(ctx context.Context, domains []*Domain, p DNSProvider, interval time.Duration, fetchers []ipaddr.IPv6Fetcher, iface, metricsAddr string) error {
	if metricsAddr != "" {
		go func() {
			if err := metrics.Serve(ctx, metricsAddr); err != nil {
				slog.Error("metrics server stopped", "module", "metrics", "err", httputil.ErrForLog(err))
			}
		}()
	}

	// 启动时立即做一次完整同步，避免等待首次 Netlink 事件或轮询周期
	slog.Info("performing initial IPv6 address fetch", "module", "ddns")
	ip, err := ipaddr.IPv6Addr(ctx, fetchers...)
	if err != nil {
		metrics.IncIPv6Fetch(false)
		// 只返回：由 CLI 边界打印，避免启动失败双打日志
		return fmt.Errorf("initial ipv6 fetch failed: %w", err)
	}
	metrics.IncIPv6Fetch(true)
	slog.Info("initial IPv6 address obtained", "module", "ddns", "ipv6", ip.String())

	// 首次同步 fail-fast：任一子域名失败则终止启动
	if err := syncAllDomains(ctx, domains, ip, p, true); err != nil {
		metrics.IncSync(false)
		return fmt.Errorf("initial sync failed: %w", err)
	}
	metrics.IncSync(true)
	metrics.MarkSuccess()

	// Linux: Netlink 事件；其他平台: 定时轮询
	triggerCh := startTrigger(ctx, interval, iface)

	slog.Info("ddns6 started successfully",
		"module", "ddns",
		"pid", os.Getpid(),
		"domain_count", len(domains),
		"mode", platformTriggerMode())

	// 跟踪进行中的同步：单飞避免密集触发叠多层；关机时 WaitGroup 等待结束
	var syncWG sync.WaitGroup
	var syncing atomic.Bool

	for {
		select {
		case <-triggerCh:
			if !syncing.CompareAndSwap(false, true) {
				metrics.IncSyncSkipped()
				slog.Debug("sync already in progress, skipping trigger", "module", "ddns")
				continue
			}
			syncWG.Go(func() {
				defer syncing.Store(false)
				runTriggeredSync(ctx, domains, p, fetchers)
			})

		case <-ctx.Done():
			slog.Info("shutdown signal received, initiating graceful shutdown...", "module", "ddns")

			done := make(chan struct{})
			go func() {
				syncWG.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				slog.Warn("graceful shutdown timed out, sync still in progress",
					"module", "ddns", "syncing", syncing.Load())
			}

			slog.Info("ddns6 stopped", "module", "ddns")
			return nil
		}
	}
}

// runTriggeredSync 执行一轮触发同步（含 sync_id 与 metrics）。
func runTriggeredSync(ctx context.Context, domains []*Domain, p DNSProvider, fetchers []ipaddr.IPv6Fetcher) {
	ctx = WithSyncID(ctx)
	syncID := SyncIDFrom(ctx)

	ip, err := ipaddr.IPv6Addr(ctx, fetchers...)
	if err != nil {
		metrics.IncIPv6Fetch(false)
		slog.ErrorContext(ctx, "failed to get IPv6 address on trigger",
			"module", "ddns", "sync_id", syncID, "err", httputil.ErrForLog(err))
		return
	}
	metrics.IncIPv6Fetch(true)

	if err := syncAllDomains(ctx, domains, ip, p, false); err != nil {
		metrics.IncSync(false)
		return
	}
	metrics.IncSync(true)
	metrics.MarkSuccess()
}

// maxSyncGroupConcurrency 同轮 sync 中并发处理的 zone 组上限。
const maxSyncGroupConcurrency = 5

// syncAllDomains 按根域名分组同步，同 zone 只查询一次 GetRecords。
//
// failFast=true 时返回第一个错误（仍等待各组结束）；failFast=false 时遇错只在组内记一次日志。
// 并发组数受 maxSyncGroupConcurrency 限制，避免瞬时打满上游 API。
func syncAllDomains(ctx context.Context, domains []*Domain, ip net.IP, p DNSProvider, failFast bool) error {
	if SyncIDFrom(ctx) == "" {
		ctx = WithSyncID(ctx)
	}

	type groupKey struct {
		root string
		typ  string
	}
	groups := make(map[groupKey][]*Domain, len(domains))
	for _, d := range domains {
		k := groupKey{root: d.Domain, typ: d.Type}
		groups[k] = append(groups[k], d)
	}

	sem := make(chan struct{}, maxSyncGroupConcurrency)
	var wg sync.WaitGroup
	var failed atomic.Bool
	errCh := make(chan error, len(groups))
	for key, group := range groups {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := syncDomainGroup(ctx, key.root, key.typ, group, ip, p); err != nil {
				failed.Store(true)
				if failFast {
					errCh <- err
				}
				// 非 failFast：错误已在 syncDomainGroup 边界记过，不再叠层
			}
		})
	}
	wg.Wait()
	close(errCh)
	if failFast {
		for err := range errCh {
			if err != nil {
				return err
			}
		}
		return nil
	}
	if failed.Load() {
		return fmt.Errorf("one or more sync groups failed")
	}
	return nil
}

// syncDomainGroup 同步同一根域名下的多个子域名：先过滤需更新项，再一次 GetRecords 后逐个 apply。
func syncDomainGroup(ctx context.Context, root, typ string, group []*Domain, ip net.IP, p DNSProvider) error {
	syncID := SyncIDFrom(ctx)
	need := make([]*Domain, 0, len(group))
	for _, d := range group {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		d.lock()
		unchanged := !hasAddressChanged(d.addr, ip)
		d.unlock()
		if unchanged {
			slog.DebugContext(ctx, "IPv6 address unchanged, skipping update", "module", "ddns",
				"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain)
			continue
		}
		need = append(need, d)
	}
	if len(need) == 0 {
		return nil
	}

	slog.DebugContext(ctx, "querying DNS records for zone", "module", "ddns",
		"sync_id", syncID, "domain", root, "record_type", typ, "subdomain_count", len(need))

	records, err := p.GetRecords(ctx, root, typ)
	if err != nil {
		slog.ErrorContext(ctx, "failed to query records",
			"module", "ddns", "sync_id", syncID, "domain", root, "record_type", typ,
			"err", httputil.ErrForLog(err))
		return fmt.Errorf("failed to query records for %s: %w", root, err)
	}

	for _, d := range need {
		if err := applyDNSRecords(ctx, d, p, ip, records); err != nil {
			slog.ErrorContext(ctx, "sync failed",
				"module", "ddns", "sync_id", syncID,
				"domain", d.Domain, "subdomain", d.SubDomain,
				"err", httputil.ErrForLog(err))
			return fmt.Errorf("sync failed for %s/%s: %w", d.Domain, d.SubDomain, err)
		}
	}
	return nil
}

// pollingLoop 按 interval 向 triggerCh 发送非阻塞触发信号。
//
// Linux 上 Netlink 不可用时回退至此；非 Linux 默认使用此模式。
func pollingLoop(ctx context.Context, triggerCh chan<- struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case triggerCh <- struct{}{}:
			default:
			}
		case <-ctx.Done():
			return
		}
	}
}
