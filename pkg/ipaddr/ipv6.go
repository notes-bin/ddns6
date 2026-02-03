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

	// 创建上下文，用于取消所有 fetcher
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建结果通道和错误通道
	resultCh := make(chan net.IP, 1)
	errCh := make(chan error, len(fetchers))

	var wg sync.WaitGroup
	for _, fn := range fetchers {
		wg.Go(func() {
			ip, err := fn.Fetch(ctx)
			if err == nil {
				resultCh <- ip
			} else {
				errCh <- err
			}
		})
	}

	go func() {
		wg.Wait()
		close(resultCh)
		close(errCh)
	}()

	select {
	case ip := <-resultCh:
		cancel() // 取消其他 fetcher
		return ip, nil
	case err := <-errCh:
		slog.Error("Fetcher failed", "error", err)
	}

	return nil, fmt.Errorf("no valid IPv6 address found from any fetcher")
}
