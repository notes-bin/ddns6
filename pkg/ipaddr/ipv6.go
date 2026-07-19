package ipaddr

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

var log = slog.With("module", "ipaddr")

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

	log.Debug("fetching IPv6 address concurrently", "fetcher_count", len(fetchers))

	resultCh := make(chan net.IP, len(fetchers))

	var wg sync.WaitGroup
	for _, fn := range fetchers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ip, err := fn.Fetch(ctx)
			if err == nil {
				log.Debug("fetcher obtained IPv6 successfully",
					"fetcher", fmt.Sprintf("%T", fn),
					"ipv6", ip.String())
				select {
				case resultCh <- ip:
				default:
				}
			} else {
				log.Warn("fetcher failed to get IPv6",
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
			log.Info("IPv6 address obtained successfully", "ipv6", ip.String())
			return ip, nil
		}
	case <-ctx.Done():
		log.Warn("IPv6 fetch timed out", "fetcher_count", len(fetchers))
		return nil, ctx.Err()
	}

	log.Error("all IPv6 fetchers failed", "fetcher_count", len(fetchers))
	return nil, fmt.Errorf("no valid IPv6 address found from any fetcher")
}
