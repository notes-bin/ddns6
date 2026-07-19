//go:build linux

package ddns

import (
	"context"
	"net"
	"time"

	"github.com/vishvananda/netlink"
)

// debounceDuration Netlink 事件的防抖等待时间。
//
// PPPoE 重拨场景下，新地址可能不会立即稳定：
//   - 重拨后内核下发新地址（RTM_NEWADDR）
//   - 但 SLAAC 可能还在进行中（临时地址生成）
//   - 10 秒的窗口足够让绝大多数场景稳定
const debounceDuration = 10 * time.Second

// startTrigger 启动 Netlink 地址监听器。
//
// 监听内核 RTM_NEWADDR 事件，当检测到新的全局单播 IPv6 地址时，
// 触发同步操作。通过 debounce 机制合并短时间内的多个事件。
//
// 如果 iface 不为空，只处理该接口的地址事件。
//
// 如果 Netlink 订阅失败（如权限不足），回退到定时轮询模式。
func startTrigger(ctx context.Context, interval time.Duration, iface string) <-chan struct{} {
	triggerCh := make(chan struct{}, 1)

	go func() {
		// 尝试建立 Netlink 订阅
		updates := make(chan netlink.AddrUpdate, 128)
		if err := netlink.AddrSubscribe(updates, ctx.Done()); err != nil {
			// Netlink 订阅失败（如权限不足），回退到定时轮询
			log.Warn("netlink subscribe failed, falling back to polling",
				"err", err, "fallback_interval", interval)
			fallbackPolling(ctx, triggerCh, interval)
			return
		}

		log.Info("netlink address listener started",
			"interface", iface, "debounce", debounceDuration)

		// 解析目标接口索引（如果指定了接口名）
		var targetIndex int
		if iface != "" {
			ifi, err := net.InterfaceByName(iface)
			if err != nil {
				log.Error("failed to resolve interface name",
					"interface", iface, "err", err)
				return
			}
			targetIndex = ifi.Index
			log.Debug("filtering netlink events by interface",
				"interface", iface, "index", targetIndex)
		}

		var debounceTimer *time.Timer
		var timerC <-chan time.Time

		for {
			select {
			case update, ok := <-updates:
				if !ok {
					log.Debug("netlink update channel closed, exiting")
					return
				}

				// 只处理新地址事件（忽略地址删除事件）
				if !update.NewAddr {
					continue
				}

				// 必须是有效的网络层地址
				if update.LinkAddress.IP == nil {
					continue
				}

				// 必须是 IPv6 地址（忽略 IPv4 事件）
				if update.LinkAddress.IP.To4() != nil {
					continue
				}

				// 必须是全局单播地址（忽略 link-local、ULA 等）
				if !update.LinkAddress.IP.IsGlobalUnicast() {
					continue
				}

				// 如果指定了接口，只处理该接口的事件
				if targetIndex > 0 && update.LinkIndex != targetIndex {
					continue
				}

				// 符合条件的新 IPv6 地址 — 重置 debounce 计时器
				if debounceTimer == nil {
					debounceTimer = time.NewTimer(debounceDuration)
					timerC = debounceTimer.C
				} else {
					debounceTimer.Stop()
					debounceTimer.Reset(debounceDuration)
				}

				evtLog := log.With(
					"interface_index", update.LinkIndex,
					"addr", update.LinkAddress.IP.String(),
				)
				evtLog.Debug("IPv6 address change detected, debounce timer reset")

			case <-timerC:
				// Debounce 时间到，地址已稳定 — 触发同步
				log.Debug("debounce timer expired, triggering DNS sync")
				select {
				case triggerCh <- struct{}{}:
				default:
				}
				debounceTimer = nil
				timerC = nil

			case <-ctx.Done():
				log.Debug("netlink trigger shutting down")
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			}
		}
	}()

	return triggerCh
}

// fallbackPolling 在 Netlink 不可用时使用定时轮询。
func fallbackPolling(ctx context.Context, triggerCh chan<- struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Info("using polling mode as fallback", "interval", interval)

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

// platformTriggerMode 返回当前平台的触发模式描述。
func platformTriggerMode() string {
	return "netlink+debounce"
}
