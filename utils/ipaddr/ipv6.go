package ipaddr

import (
	"context"
	"net"
	"sync"
)

// IPv6Provider 定义获取 IPv6 地址的接口
type IPv6Provider interface {
	GetIPv6Addr() (net.IP, error)
}

// MultiProvider 多 IPv6 地址提供者
type MultiProvider struct {
	providers []IPv6Provider
}

// NewMultiProvider 创建多 IPv6 地址提供者
func NewMultiProvider(providers ...IPv6Provider) *MultiProvider {
	return &MultiProvider{providers: providers}
}

// GetIPv6Addr 从多个来源并发获取 IPv6 地址，返回第一个有效结果
func (m *MultiProvider) GetIPv6Addr(ctx context.Context) (net.IP, error) {
	var wg sync.WaitGroup
	resultChan := make(chan net.IP, 1)
	errChan := make(chan error, len(m.providers))

	for _, provider := range m.providers {
		wg.Add(1)
		go func(p IPv6Provider) {
			defer wg.Done()
			ip, err := p.GetIPv6Addr()
			if err != nil {
				errChan <- err
				return
			}
			select {
			case resultChan <- ip:
				// 关闭 errChan 以避免 goroutine 泄漏
				close(errChan)
			case <-ctx.Done():
				return
			}
		}(provider)
	}

	// 启动一个 goroutine 等待所有任务完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 返回第一个有效结果
	select {
	case ip, ok := <-resultChan:
		if ok {
			return ip, nil
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 如果没有有效结果，返回第一个错误
	return nil, <-errChan
}
