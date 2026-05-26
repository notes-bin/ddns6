package ipaddr

import (
	"context"
	"fmt"
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

	resultCh := make(chan net.IP, len(fetchers))

	var wg sync.WaitGroup
	for _, fn := range fetchers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ip, err := fn.Fetch(ctx)
			if err == nil {
				select {
				case resultCh <- ip:
				default:
				}
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
			return ip, nil
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return nil, fmt.Errorf("no valid IPv6 address found from any fetcher")
}
