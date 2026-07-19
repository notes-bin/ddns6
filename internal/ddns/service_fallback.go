//go:build !linux

package ddns

import (
	"context"
	"time"
)

// startTrigger 启动定时轮询地址变化。
//
// 非 Linux 平台不支持 Netlink，使用 time.NewTicker 定期检查地址变化。
// interval 由用户通过 --interval 参数控制，默认 5 分钟。
func startTrigger(ctx context.Context, interval time.Duration, iface string) <-chan struct{} {
	triggerCh := make(chan struct{}, 1)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Info("using polling mode",
			"interval", interval,
			"platform", "non-linux")

		for {
			select {
			case <-ticker.C:
				log.Debug("polling timer triggered, fetching IPv6 address")
				select {
				case triggerCh <- struct{}{}:
				default:
				}
			case <-ctx.Done():
				log.Debug("polling trigger shutting down")
				return
			}
		}
	}()

	return triggerCh
}

// platformTriggerMode 返回当前平台的触发模式描述。
func platformTriggerMode() string {
	return "polling"
}
