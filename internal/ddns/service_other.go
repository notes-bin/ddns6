//go:build !linux

package ddns

// 本文件实现非 Linux 平台的地址变化触发：按 --interval 定时轮询。

import (
	"context"
	"log/slog"
	"time"
)

// startTrigger 启动定时轮询地址变化触发器。
//
// 非 Linux 不支持 Netlink，使用 ticker 定期检查；interval 由 --interval 控制，默认 5 分钟。
func startTrigger(ctx context.Context, interval time.Duration, _ string) <-chan struct{} {
	triggerCh := make(chan struct{}, 1)

	go func() {
		slog.Info("using polling mode",
			"module", "ddns",
			"interval", interval,
			"platform", "non-linux")

		pollingLoop(ctx, triggerCh, interval)
	}()

	return triggerCh
}

// platformTriggerMode 返回当前平台的触发模式描述（日志用）。
func platformTriggerMode() string {
	return "polling"
}
