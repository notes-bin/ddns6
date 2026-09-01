package ddns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// DefaultIPv6Fetchers 返回默认的 IPv6 地址获取器列表（每次调用返回新切片，避免调用方污染全局状态）。
//
// 每次触发同步时由 GetIPv6Addr 随机打乱顺序后并发竞速，取第一个成功结果。
// 包含 HTTP 与 DNS 两种来源，互为备份。
func DefaultIPv6Fetchers() []ipaddr.IPv6Fetcher {
	return []ipaddr.IPv6Fetcher{
		ipaddr.NewHttpIPv6Fetcher("https://6.ipw.cn"),
		ipaddr.NewHttpIPv6Fetcher("https://ifconfig.co"),
		ipaddr.NewHttpIPv6Fetcher("https://v6.ident.me"),
		ipaddr.NewDnsFetcher("2402:4e00::"),
		ipaddr.NewDnsFetcher("2400:3200:baba::1"),
		ipaddr.NewDnsFetcher("2001:4860:4860::8888"),
		ipaddr.NewDnsFetcher("2606:4700:4700::1111"),
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
//   - 收到 SIGINT 或 SIGTERM 后优雅关闭
//   - 先取消正在进行的操作，再等待最多 5 秒让当前同步完成
//   - 然后返回 nil
func RunService(domains []*Domain, p DNSProvider, interval time.Duration, fetchers []ipaddr.IPv6Fetcher, iface string) error {
	slog.Info("starting DDNS update service",
		"module", "ddns",
		"domain_count", len(domains),
		"interval", interval,
		"interface", iface)

	// 可取消 context：SIGTERM 时 cancel 会传播到进行中的获取与同步
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动时立即做一次完整同步，避免等待首次 Netlink 事件或轮询周期
	slog.Info("performing initial IPv6 address fetch", "module", "ddns")
	ip, err := ipaddr.GetIPv6Addr(ctx, fetchers...)
	if err != nil {
		return fmt.Errorf("initial IPv6 fetch failed: %w", err)
	}
	slog.Info("initial IPv6 address obtained", "module", "ddns", "ipv6", ip.String())

	// 首次同步 fail-fast：任一子域名失败则终止启动
	if err := syncAllDomains(ctx, domains, ip, p, true); err != nil {
		return err
	}

	// Linux: Netlink 事件；其他平台: 定时轮询
	triggerCh := startTrigger(ctx, interval, iface)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	slog.Info("ddns6 started successfully",
		"module", "ddns",
		"pid", os.Getpid(),
		"domain_count", len(domains),
		"mode", platformTriggerMode())

	// 同步在独立 goroutine 中执行，保证 sigCh 始终可达
	syncDoneCh := make(chan struct{}, 1)

	for {
		select {
		case <-triggerCh:
			go func() {
				ip, err := ipaddr.GetIPv6Addr(ctx, fetchers...)
				if err != nil {
					slog.Error("failed to get IPv6 address on trigger", "module", "ddns", "err", err)
					syncDoneCh <- struct{}{}
					return
				}
				syncAllDomains(ctx, domains, ip, p, false)
				syncDoneCh <- struct{}{}
			}()

		case <-syncDoneCh:
			// 本轮同步结束，继续等待下一事件

		case <-sigCh:
			slog.Info("shutdown signal received, initiating graceful shutdown...", "module", "ddns")
			cancel()

			select {
			case <-time.After(5 * time.Second):
				slog.Warn("graceful shutdown timed out", "module", "ddns")
			}

			signal.Stop(sigCh)
			slog.Info("ddns6 stopped", "module", "ddns")
			return nil

		}
	}
}

// syncAllDomains 并发同步所有域名的 DNS 记录。
//
// failFast=true 时遇错立即返回第一个错误；failFast=false 时遇错只记日志并继续处理剩余域名。
func syncAllDomains(ctx context.Context, domains []*Domain, ip net.IP, p DNSProvider, failFast bool) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(domains))
	for _, d := range domains {
		wg.Go(func() {
			if err := SyncRecord(ctx, d, ip, p); err != nil {
				if failFast {
					errCh <- fmt.Errorf("sync failed for %s/%s: %w",
						d.Domain, d.SubDomain, err)
				} else {
					slog.Error("sync failed on trigger",
						"module", "ddns",
						"domain", d.Domain, "subdomain", d.SubDomain, "err", err)
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
