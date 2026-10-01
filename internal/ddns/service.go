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
func RunService(domains []*Domain, p DNSProvider, interval time.Duration, fetchers []ipaddr.IPv6Fetcher, iface string) error {
	slog.Info("starting DDNS update service",
		"module", "ddns",
		"domain_count", len(domains),
		"interval", interval,
		"interface", iface)

	// 可取消 context：SIGINT/SIGTERM 时取消并传播到进行中的获取与同步
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 启动时立即做一次完整同步，避免等待首次 Netlink 事件或轮询周期
	slog.Info("performing initial IPv6 address fetch", "module", "ddns")
	ip, err := ipaddr.IPv6Addr(ctx, fetchers...)
	if err != nil {
		return fmt.Errorf("initial ipv6 fetch failed: %w", err)
	}
	slog.Info("initial IPv6 address obtained", "module", "ddns", "ipv6", ip.String())

	// 首次同步 fail-fast：任一子域名失败则终止启动
	if err := syncAllDomains(ctx, domains, ip, p, true); err != nil {
		return err
	}

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
				slog.Debug("sync already in progress, skipping trigger", "module", "ddns")
				continue
			}
			syncWG.Go(func() {
				defer syncing.Store(false)
				ip, err := ipaddr.IPv6Addr(ctx, fetchers...)
				if err != nil {
					slog.Error("failed to get IPv6 address on trigger", "module", "ddns", "err", err)
					return
				}
				syncAllDomains(ctx, domains, ip, p, false)
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
				slog.Warn("graceful shutdown timed out", "module", "ddns")
			}

			slog.Info("ddns6 stopped", "module", "ddns")
			return nil
		}
	}
}

// syncAllDomains 按根域名分组同步，同 zone 只查询一次 GetRecords。
//
// failFast=true 时返回第一个错误（仍等待各组结束）；failFast=false 时遇错只记日志。
func syncAllDomains(ctx context.Context, domains []*Domain, ip net.IP, p DNSProvider, failFast bool) error {
	type groupKey struct {
		root string
		typ  string
	}
	groups := make(map[groupKey][]*Domain, len(domains))
	for _, d := range domains {
		k := groupKey{root: d.Domain, typ: d.Type}
		groups[k] = append(groups[k], d)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(groups))
	for key, group := range groups {
		wg.Go(func() {
			if err := syncDomainGroup(ctx, key.root, key.typ, group, ip, p); err != nil {
				if failFast {
					errCh <- err
				} else {
					slog.Error("sync group failed on trigger",
						"module", "ddns",
						"domain", key.root, "type", key.typ, "err", err)
				}
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
	}
	return nil
}

// syncDomainGroup 同步同一根域名下的多个子域名：先过滤需更新项，再一次 GetRecords 后逐个 apply。
func syncDomainGroup(ctx context.Context, root, typ string, group []*Domain, ip net.IP, p DNSProvider) error {
	need := make([]*Domain, 0, len(group))
	for _, d := range group {
		d.lock()
		select {
		case <-ctx.Done():
			d.unlock()
			return ctx.Err()
		default:
		}
		if !hasAddressChanged(d.Addr, ip) {
			slog.Debug("IPv6 address unchanged, skipping update", "module", "ddns",
				"domain", d.Domain, "subdomain", d.SubDomain)
			d.unlock()
			continue
		}
		d.unlock()
		need = append(need, d)
	}
	if len(need) == 0 {
		return nil
	}

	slog.Debug("querying DNS records for zone", "module", "ddns",
		"domain", root, "type", typ, "subdomain_count", len(need))

	records, err := p.GetRecords(ctx, root, typ)
	if err != nil {
		return fmt.Errorf("failed to query records for %s: %w", root, err)
	}

	for _, d := range need {
		d.lock()
		err := applyDNSRecords(ctx, d, p, ip, records)
		d.unlock()
		if err != nil {
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
