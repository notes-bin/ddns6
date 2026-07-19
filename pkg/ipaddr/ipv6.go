// Package ipaddr 提供本机 IPv6 地址获取功能。
//
// 获取策略（每次调用时随机排序）：
//   1. 随机打乱所有 fetcher 的顺序
//   2. 按乱序逐个尝试每个 fetcher
//   3. 第一个成功返回的地址即为结果
//   4. 全部失败则返回错误
package ipaddr

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"time"
)

var log = slog.With("module", "ipaddr")

// IPv6Fetcher 定义了获取 IPv6 地址的接口
type IPv6Fetcher interface {
	Fetch(ctx context.Context) (net.IP, error)
}

// GetIPv6Addr 获取本机 IPv6 地址。
//
// 每次调用都会随机打乱 fetchers 顺序后逐个尝试，
// 第一个成功返回的地址即作为结果。全部失败则返回错误。
// 总超时时间为 5 秒。
func GetIPv6Addr(fetchers ...IPv6Fetcher) (net.IP, error) {
	if len(fetchers) == 0 {
		return nil, fmt.Errorf("no fetcher provided")
	}

	// 随机打乱 fetchers 顺序，避免对某个源产生固定依赖
	shuffled := make([]IPv6Fetcher, len(fetchers))
	copy(shuffled, fetchers)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	log.Debug("attempting to fetch IPv6 address", "fetcher_count", len(fetchers))

	// 总超时 5 秒
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lastErr error
	for i, fn := range shuffled {
		// 每次尝试前检查 context 是否已超时
		select {
		case <-ctx.Done():
			log.Warn("IPv6 fetch timed out",
				"tried", i, "total", len(fetchers))
			return nil, fmt.Errorf("fetch timeout after %d/%d attempts: %w", i, len(fetchers), ctx.Err())
		default:
		}

		log.Debug("trying fetcher",
			"attempt", i+1, "total", len(fetchers),
			"fetcher", fmt.Sprintf("%T", fn))

		ip, err := fn.Fetch(ctx)
		if err != nil {
			log.Warn("fetcher failed",
				"fetcher", fmt.Sprintf("%T", fn),
				"attempt", i+1, "err", err)
			lastErr = err
			continue
		}

		log.Info("IPv6 address obtained successfully",
			"fetcher", fmt.Sprintf("%T", fn),
			"ipv6", ip.String(),
			"attempt", i+1)
		return ip, nil
	}

	log.Error("all IPv6 fetchers failed",
		"total", len(fetchers), "last_err", lastErr)
	return nil, fmt.Errorf("all %d fetchers failed: %w", len(fetchers), lastErr)
}
