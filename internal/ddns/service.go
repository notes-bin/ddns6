// Package ddns 提供动态域名解析（DDNS）服务编排。
//
// 工作流程：
//
//	Linux: Netlink 事件监听 → debounce 10s → 获取 IPv6 → 同步 DNS 记录
//	其他:  cron 定时轮询 → 获取 IPv6 → 同步 DNS 记录
//
// RunService 是唯一的公开入口，接受域名列表、DNS 服务商等参数。
// 同一个进程可以管理同一根域名下的多个子域名。
package ddns

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/internal/providers"
	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

var log = slog.With("module", "ddns")

// DefaultIPv6Fetchers 默认的 IPv6 地址获取器列表。
//
// 每次触发同步时，会随机打乱此列表后逐个尝试，取第一个成功的结果。
// 包含 HTTP 和 DNS 两种获取方式，互为备份。
var DefaultIPv6Fetchers = []ipaddr.IPv6Fetcher{
	ipaddr.NewHttpIPv6Fetcher("6.ipw.cn"),
	ipaddr.NewHttpIPv6Fetcher("ifconfig.co"),
	ipaddr.NewHttpIPv6Fetcher("v6.ident.me"),
	ipaddr.NewDnsFetcher("2402:4e00::"),
	ipaddr.NewDnsFetcher("2400:3200:baba::1"),
	ipaddr.NewDnsFetcher("2001:4860:4860::8888"),
	ipaddr.NewDnsFetcher("2606:4700:4700::1111"),
}

// RunService 启动 DDNS 服务，持续监听 IPv6 地址变化并更新 DNS 记录。
//
// 参数:
//   - domains: 要更新的域名列表（支持同一根域名下多个子域名）
//   - p: DNS 服务商实现
//   - interval: 非 Linux 平台的轮询间隔（Linux 下由 Netlink 事件驱动，此参数无效）
//   - fetchers: IPv6 地址获取器列表，每次触发时随机顺序逐个尝试
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
func RunService(domains []*providers.Domain, p providers.DNSProvider, interval time.Duration, fetchers []ipaddr.IPv6Fetcher, iface string) error {
	log.Info("starting DDNS update service",
		"domain_count", len(domains),
		"interval", interval,
		"interface", iface)

	// 创建一个可取消的 context 用于优雅关闭
	// 收到 SIGTERM 时 cancel 会传播到所有正在进行的操作
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ============================================================
	// 首次同步
	// 启动时就进行一次完整的 IPv6 获取 + DNS 同步。
	// 这样用户无需等待第一次 Netlink 事件或轮询周期。
	// ============================================================
	log.Info("performing initial IPv6 address fetch")
	ip, err := ipaddr.GetIPv6Addr(fetchers...)
	if err != nil {
		return fmt.Errorf("initial IPv6 fetch failed: %w", err)
	}
	log.Info("initial IPv6 address obtained", "ipv6", ip.String())

	for _, d := range domains {
		if err := SyncRecord(ctx, d, ip, p); err != nil {
			// 首次同步失败作为错误返回，确保用户能及时发现配置问题
			return fmt.Errorf("initial sync failed for %s/%s: %w",
				d.Domain, d.SubDomain, err)
		}
	}

	// ============================================================
	// 启动地址变化触发源
	// Linux: Netlink 事件监听（实时）
	// 其他: 定时轮询（简单可靠）
	// ============================================================
	triggerCh := startTrigger(ctx, interval, iface)

	// ============================================================
	// 信号处理
	// 监听 SIGINT (Ctrl+C) 和 SIGTERM (kill) 实现优雅关闭
	// ============================================================
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	log.Info("ddns6 started successfully",
		"pid", os.Getpid(),
		"domain_count", len(domains),
		"mode", triggerMode())

	// ============================================================
	// 主事件循环
	// 等待触发器事件或退出信号
	// ============================================================
	for {
		select {
		case <-triggerCh:
			// 触发器事件：重新获取 IPv6 地址并同步所有子域名
			ip, err := ipaddr.GetIPv6Addr(fetchers...)
			if err != nil {
				log.Error("failed to get IPv6 address on trigger", "err", err)
				continue
			}
			for _, d := range domains {
				if err := SyncRecord(ctx, d, ip, p); err != nil {
					log.Error("sync failed on trigger",
						"domain", d.Domain, "subdomain", d.SubDomain, "err", err)
				}
			}

		case <-sigCh:
			// 收到退出信号，开始优雅关闭
			log.Info("shutdown signal received, initiating graceful shutdown...")
			cancel() // 取消正在进行的操作

			// 给正在执行的同步操作最多 5 秒的完成时间
			// 5 秒后无论是否完成都强制退出
			time.Sleep(5 * time.Second)

			log.Info("ddns6 stopped")
			return nil

		case <-ctx.Done():
			// context 被取消（正常情况下不会走到这里，由 sigCh 分支处理）
			log.Info("context cancelled, shutting down")
			return nil
		}
	}
}

// triggerMode 返回当前平台使用的触发模式名称（仅用于日志）。
func triggerMode() string {
	return platformTriggerMode()
}
