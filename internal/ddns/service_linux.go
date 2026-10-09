//go:build linux

package ddns

// 本文件实现 Linux 平台的地址变化触发：Netlink RTM_NEWADDR + 防抖，失败时回退轮询。

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/notes-bin/ddns6/pkg/ipaddr"
	"github.com/vishvananda/netlink"
)

// debounceDuration 为 Netlink 事件的防抖等待时间。
//
// PPPoE 重拨后新地址可能尚未稳定（SLAAC/临时地址仍在生成）；
// 10 秒窗口足以覆盖绝大多数场景。
const debounceDuration = 10 * time.Second

// startTrigger 启动 Netlink 地址监听器。
//
// 监听内核 RTM_NEWADDR；检测到新的公网 IPv6 后经 debounce 合并短时多次事件再触发同步。
// iface 非空时只处理该接口。订阅失败（如权限不足）时回退到定时轮询。
func startTrigger(ctx context.Context, interval time.Duration, iface string) <-chan struct{} {
	triggerCh := make(chan struct{}, 1)

	go func() {
		// 先解析接口再订阅，避免订阅成功后回退时遗弃 updates 导致接收 goroutine 阻塞
		var targetIndex int
		if iface != "" {
			ifi, err := net.InterfaceByName(iface)
			if err != nil {
				slog.Error("failed to resolve interface name, falling back to polling",
					"module", "ddns", "interface", iface, "err", err,
					"fallback_interval", interval)
				fallbackPolling(ctx, triggerCh, interval)
				return
			}
			targetIndex = ifi.Index
			slog.Debug("filtering netlink events by interface",
				"module", "ddns", "interface", iface, "index", targetIndex)
		}

		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		updates := make(chan netlink.AddrUpdate, 128)
		if err := netlink.AddrSubscribe(updates, subCtx.Done()); err != nil {
			slog.Warn("netlink subscribe failed, falling back to polling",
				"module", "ddns", "err", err, "fallback_interval", interval)
			fallbackPolling(ctx, triggerCh, interval)
			return
		}

		slog.Info("netlink address listener started",
			"module", "ddns", "interface", iface, "debounce", debounceDuration)

		var debounceTimer *time.Timer
		var timerC <-chan time.Time

		for {
			select {
			case update, ok := <-updates:
				if !ok {
					slog.Warn("netlink update channel closed, falling back to polling",
						"module", "ddns", "fallback_interval", interval)
					cancel()
					fallbackPolling(ctx, triggerCh, interval)
					return
				}

				if !update.NewAddr || update.LinkAddress.IP == nil {
					continue
				}

				// 忽略 IPv4、link-local、ULA 等非公网地址
				if !ipaddr.IsPublicIPv6(update.LinkAddress.IP) {
					continue
				}

				if targetIndex > 0 && update.LinkIndex != targetIndex {
					continue
				}

				if debounceTimer == nil {
					debounceTimer = time.NewTimer(debounceDuration)
					timerC = debounceTimer.C
				} else {
					if !debounceTimer.Stop() {
						select {
						case <-debounceTimer.C:
						default:
						}
					}
					debounceTimer.Reset(debounceDuration)
				}

				slog.Debug("IPv6 address change detected, debounce timer reset",
					"module", "ddns",
					"interface_index", update.LinkIndex,
					"addr", update.LinkAddress.IP.String())

			case <-timerC:
				slog.Debug("debounce timer expired, triggering DNS sync",
					"module", "ddns")
				select {
				case triggerCh <- struct{}{}:
				default:
				}
				debounceTimer = nil
				timerC = nil

			case <-ctx.Done():
				slog.Debug("netlink trigger shutting down",
					"module", "ddns")
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			}
		}
	}()

	return triggerCh
}

// fallbackPolling 在 Netlink 不可用时改用定时轮询。
func fallbackPolling(ctx context.Context, triggerCh chan<- struct{}, interval time.Duration) {
	slog.Info("using polling mode as fallback", "module", "ddns", "interval", interval)
	pollingLoop(ctx, triggerCh, interval)
}

// platformTriggerMode 返回当前平台的触发模式描述（日志用）。
func platformTriggerMode() string {
	return "netlink+debounce"
}
