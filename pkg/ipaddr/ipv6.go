// Package ipaddr 提供本机公网 IPv6 地址获取。
//
// 入口为 GetIPv6Addr：对多个 IPv6Fetcher 随机打乱后并发竞速，
// 取首个成功结果。内置 HttpIPv6Fetcher（HTTP 纯文本端点）与
// DnsFetcher（UDP6 拨号取本地地址）；库调用方也可传入
// ddns.DefaultIPv6Fetchers()。
//
// 使用示例：
//
//	ctx := context.Background()
//	ip, err := ipaddr.GetIPv6Addr(ctx,
//	    ipaddr.NewHttpIPv6Fetcher("https://6.ipw.cn"),
//	    ipaddr.NewDnsFetcher("2001:4860:4860::8888"),
//	)
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

// IPv6Fetcher 定义获取本机 IPv6 地址的接口；实现须尊重 ctx 取消与超时。
type IPv6Fetcher interface {
	Fetch(ctx context.Context) (net.IP, error)
}

// fetchTimeout 单次 GetIPv6Addr 竞速的总超时。
const fetchTimeout = 5 * time.Second

// GetIPv6Addr 获取本机 IPv6 地址。
//
// 每次调用随机打乱 fetchers 后并发执行，返回第一个成功地址；
// 全部失败则返回错误。总超时 5 秒，并受父 context 约束。
func GetIPv6Addr(ctx context.Context, fetchers ...IPv6Fetcher) (net.IP, error) {
	if len(fetchers) == 0 {
		return nil, fmt.Errorf("no fetcher provided")
	}

	// 随机顺序，避免长期偏倚某一上游
	shuffled := make([]IPv6Fetcher, len(fetchers))
	copy(shuffled, fetchers)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	slog.Debug("attempting to fetch IPv6 address", "module", "ipaddr", "fetcher_count", len(fetchers))

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	resultCh := make(chan net.IP, len(shuffled))
	errCh := make(chan error, len(shuffled))

	for _, fn := range shuffled {
		go func() {
			slog.Debug("starting fetcher", "module", "ipaddr", "fetcher", fmt.Sprintf("%T", fn))
			ip, err := fn.Fetch(ctx)
			if err != nil {
				// 取消多为竞速副作用；超时与其它错误才值得关注
				switch {
				case errors.Is(err, context.Canceled):
					slog.Debug("fetcher canceled", "module", "ipaddr", "fetcher", fmt.Sprintf("%T", fn))
				case errors.Is(err, context.DeadlineExceeded):
					slog.Info("fetcher timed out", "module", "ipaddr", "fetcher", fmt.Sprintf("%T", fn))
				default:
					slog.Warn("fetcher failed", "module", "ipaddr", "fetcher", fmt.Sprintf("%T", fn), "err", err)
				}
				errCh <- err
				return
			}
			resultCh <- ip
		}()
	}

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
