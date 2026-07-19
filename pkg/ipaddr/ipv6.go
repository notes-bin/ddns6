// Package ipaddr 提供本机 IPv6 地址获取功能。
//
// 获取策略（每次调用时随机排序后并发竞速）：
//  1. 随机打乱所有 fetcher 的顺序
//  2. 所有 fetcher 并发执行
//  3. 第一个成功返回的地址即为结果
//  4. 全部失败则返回错误
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
// 每次调用都会随机打乱 fetchers 顺序后并发执行，
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

	// 并发竞速：所有 fetcher 同时启动，第一个成功返回的即为结果
	resultCh := make(chan net.IP, len(shuffled))
	errCh := make(chan error, len(shuffled))

	for _, fn := range shuffled {
		go func() {
			log.Debug("starting fetcher", "fetcher", fmt.Sprintf("%T", fn))
			ip, err := fn.Fetch(ctx)
			if err != nil {
				log.Warn("fetcher failed", "fetcher", fmt.Sprintf("%T", fn), "err", err)
				errCh <- err
				return
			}
			resultCh <- ip
		}()
	}

	// 等待第一个成功结果或所有失败
	var lastErr error
	remaining := len(shuffled)
	for remaining > 0 {
		select {
		case ip := <-resultCh:
			log.Info("IPv6 address obtained successfully",
				"ipv6", ip.String(),
				"remaining", remaining-1)
			return ip, nil
		case err := <-errCh:
			lastErr = err
			remaining--
		}
	}

	log.Error("all IPv6 fetchers failed",
		"total", len(fetchers), "last_err", lastErr)
	return nil, fmt.Errorf("all %d fetchers failed: %w", len(fetchers), lastErr)
}
