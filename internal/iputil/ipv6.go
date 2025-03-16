package iputil

import (
	"context"
	"net"
	"sync"
)

// IPv6Provider 定义获取 IPv6 地址的接口
type IPv6Provider interface {
	GetIPv6Addr() (net.IP, error)
}

// GetIPv6Addr 从多个来源并发获取 IPv6 地址，返回第一个有效结果
func GetIPv6Addr(ctx context.Context) (net.IP, error) {
	providers := []IPv6Provider{
		NewIfaceProvider(),
		NewSiteProvider(),
		NewDNSProvider(),
	}

	var wg sync.WaitGroup
	results := make(chan net.IP, len(providers))
	errs := make(chan error, len(providers))

	for _, provider := range providers {
		wg.Add(1)
		go func(p IPv6Provider) {
			defer wg.Done()
			ip, err := p.GetIPv6Addr()
			if err != nil {
				errs <- err
				return
			}
			select {
			case results <- ip:
			case <-ctx.Done():
			}
		}(provider)
	}

	// 等待所有任务完成
	wg.Wait()
	close(results)
	close(errs)

	// 返回第一个有效结果
	select {
	case ip := <-results:
		return ip, nil
	default:
		return nil, <-errs
	}
}
