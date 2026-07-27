// Package ipaddr 提供本机 IPv6 地址获取功能。
//
// 使用示例：
//
//	// 使用默认获取器（3 个 HTTP + 4 个 DNS 源）
//	ip, err := ipaddr.GetIPv6Addr(ipaddr.DefaultFetchers...)
//
//	// 自定义获取器
//	fetcher := ipaddr.NewHttpIPv6Fetcher("https://6.ipw.cn")
//	ip, err := ipaddr.GetIPv6Addr(fetcher)
//
// 获取策略（每次调用时随机排序后并发竞速）：
//  1. 随机打乱所有 fetcher 的顺序
//  2. 所有 fetcher 并发执行
//  3. 第一个成功返回的地址即为结果
//  4. 全部失败则返回错误
package ipaddr

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"time"
)

// IPv6Fetcher 定义了获取 IPv6 地址的接口
type IPv6Fetcher interface {
	Fetch(ctx context.Context) (net.IP, error)
}

// fetchTimeout IPv6 地址获取总超时时间。
const fetchTimeout = 5 * time.Second

// GetIPv6Addr 获取本机 IPv6 地址。
//
// 每次调用都会随机打乱 fetchers 顺序后并发执行，
// 第一个成功返回的地址即作为结果。全部失败则返回错误。
// 总超时时间为 5 秒，受父 context 控制。
func GetIPv6Addr(ctx context.Context, fetchers ...IPv6Fetcher) (net.IP, error) {
	if len(fetchers) == 0 {
		return nil, fmt.Errorf("no fetcher provided")
	}

	// 随机打乱 fetchers 顺序，避免对某个源产生固定依赖
	shuffled := make([]IPv6Fetcher, len(fetchers))
	copy(shuffled, fetchers)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	slog.Debug("attempting to fetch IPv6 address", "module", "ipaddr", "fetcher_count", len(fetchers))

	// 总超时 5 秒，继承父 context 以支持优雅关闭
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	// 并发竞速：所有 fetcher 同时启动，第一个成功返回的即为结果
	resultCh := make(chan net.IP, len(shuffled))
	errCh := make(chan error, len(shuffled))

	for _, fn := range shuffled {
		go func(fetcher IPv6Fetcher) {
			slog.Debug("starting fetcher", "module", "ipaddr", "fetcher", fmt.Sprintf("%T", fetcher))
			ip, err := fetcher.Fetch(ctx)
			if err != nil {
				// 区分错误类型：取消=竞速正常副作用，超时=可能网络问题，其他=真正故障
				switch {
				case errors.Is(err, context.Canceled):
					slog.Debug("fetcher canceled", "module", "ipaddr", "fetcher", fmt.Sprintf("%T", fetcher))
				case errors.Is(err, context.DeadlineExceeded):
					slog.Info("fetcher timed out", "module", "ipaddr", "fetcher", fmt.Sprintf("%T", fetcher))
				default:
					slog.Warn("fetcher failed", "module", "ipaddr", "fetcher", fmt.Sprintf("%T", fetcher), "err", err)
				}
				errCh <- err
				return
			}
			resultCh <- ip
		}(fn)
	}

	// 等待第一个成功结果或所有失败，同时统计各类错误数量
	var lastErr error
	var canceledCount, timeoutCount, failedCount int
	remaining := len(shuffled)
	for remaining > 0 {
		select {
		case ip := <-resultCh:
			slog.Info("IPv6 address obtained successfully",
				"module", "ipaddr",
				"ipv6", ip.String(),
				"canceled", canceledCount, "timed_out", timeoutCount, "failed", failedCount)
			return ip, nil
		case err := <-errCh:
			lastErr = err
			remaining--
			switch {
			case errors.Is(err, context.Canceled):
				canceledCount++
			case errors.Is(err, context.DeadlineExceeded):
				timeoutCount++
			default:
				failedCount++
			}
		}
	}

	slog.Error("all IPv6 fetchers failed",
		"module", "ipaddr",
		"total", len(fetchers),
		"canceled", canceledCount, "timed_out", timeoutCount, "failed", failedCount,
		"last_err", lastErr)
	return nil, fmt.Errorf("all %d fetchers failed: %w", len(fetchers), lastErr)
}
