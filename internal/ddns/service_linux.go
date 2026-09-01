//go:build linux

package ddns

// 本文件实现 Linux 平台的地址变化触发：Netlink RTM_NEWADDR + 防抖，失败时回退轮询。

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/vishvananda/netlink"
)

// debounceDuration 为 Netlink 事件的防抖等待时间。
//
// PPPoE 重拨后新地址可能尚未稳定（SLAAC/临时地址仍在生成）；
// 10 秒窗口足以覆盖绝大多数场景。
const debounceDuration = 10 * time.Second

// startTrigger 启动 Netlink 地址监听器。
//
// 监听内核 RTM_NEWADDR；检测到新的全局单播 IPv6 后经 debounce 合并短时多次事件再触发同步。
// iface 非空时只处理该接口。订阅失败（如权限不足）时回退到定时轮询。
func startTrigger(ctx context.Context, interval time.Duration, iface string) <-chan struct{} {
	triggerCh := make(chan struct{}, 1)

	go func() {
		updates := make(chan netlink.AddrUpdate, 128)
		if err := netlink.AddrSubscribe(updates, ctx.Done()); err != nil {
			slog.Warn("netlink subscribe failed, falling back to polling",
				"module", "ddns", "err", err, "fallback_interval", interval)
			fallbackPolling(ctx, triggerCh, interval)
			return
		}

		slog.Info("netlink address listener started",
			"module", "ddns", "interface", iface, "debounce", debounceDuration)

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

		var debounceTimer *time.Timer
		var timerC <-chan time.Time

		for {
			select {
			case update, ok := <-updates:
				if !ok {
					slog.Warn("netlink update channel closed, falling back to polling",
						"module", "ddns", "fallback_interval", interval)
					fallbackPolling(ctx, triggerCh, interval)
					return
				}

				// 忽略地址删除，只关心新增
				if !update.NewAddr {
					continue
				}

				if update.LinkAddress.IP == nil {
					continue
				}

				// 忽略 IPv4 与非全局单播（link-local、ULA 等）
				if update.LinkAddress.IP.To4() != nil {
					continue
				}
				if !update.LinkAddress.IP.IsGlobalUnicast() {
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
						// 排空已触发的 channel，避免 Reset 后立即误触发
						select {
						case <-debounceTimer.C:
						default:
						}
					}
					debounceTimer.Reset(debounceDuration)
				}

				evtLog := slog.With(
					"interface_index", update.LinkIndex,
					"addr", update.LinkAddress.IP.String(),
				)
				evtLog.Debug("IPv6 address change detected, debounce timer reset")

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
