package ipaddr

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// IPv6Fetcher 定义了获取 IPv6 地址的接口
type IPv6Fetcher interface {
	Fetch(ctx context.Context) (net.IP, error)
}

// GetIPv6Addr 获取第一个成功返回的 IPv6 地址
func GetIPv6Addr(fetchers ...IPv6Fetcher) (net.IP, error) {
	if len(fetchers) == 0 {
		return nil, fmt.Errorf("no fetcher provided")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slog.Debug("开始并行获取 IPv6 地址", "fetcher_count", len(fetchers))

	resultCh := make(chan net.IP, len(fetchers))

	var wg sync.WaitGroup
	for _, fn := range fetchers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ip, err := fn.Fetch(ctx)
			if err == nil {
				slog.Debug("fetcher 获取 IPv6 成功",
					"fetcher", fmt.Sprintf("%T", fn),
					"ipv6", ip.String())
				select {
				case resultCh <- ip:
				default:
				}
			} else {
				slog.Warn("fetcher 获取 IPv6 失败",
					"fetcher", fmt.Sprintf("%T", fn),
					"err", err)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	select {
	case ip, ok := <-resultCh:
		if ok {
			cancel()
			slog.Info("IPv6 地址获取成功", "ipv6", ip.String())
			return ip, nil
		}
	case <-ctx.Done():
		slog.Warn("IPv6 获取超时", "fetcher_count", len(fetchers))
		return nil, ctx.Err()
	}

	slog.Error("所有 IPv6 fetcher 均失败", "fetcher_count", len(fetchers))
	return nil, fmt.Errorf("no valid IPv6 address found from any fetcher")
}
