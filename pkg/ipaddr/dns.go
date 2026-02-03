package ipaddr

import (
	"context"
	"fmt"
	"net"
)

// DnsFetcher 从 DNS 服务器获取 IPv6 地址
type DnsFetcher string

// NewDnsFetcher 创建新的 DnsFetcher
func NewDnsFetcher(server string) *DnsFetcher {
	return (*DnsFetcher)(&server)
}

// String 返回 DnsFetcher 的字符串表示
func (d *DnsFetcher) String() string {
	return string(*d)
}

// Fetch 实现 Fetcher 接口
func (d *DnsFetcher) Fetch(ctx context.Context) (net.IP, error) {
	// 创建 DNS 查询
	conn, err := net.Dial("udp6", fmt.Sprintf("[%s]:53", *d))
	if err != nil {
		return nil, fmt.Errorf("failed to dial DNS server %s: %w", *d, err)
	}
	defer conn.Close()

	// 获取本地地址
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	if localAddr.IP.To16() != nil && localAddr.IP.To4() == nil {
		return localAddr.IP, nil
	}

	return nil, fmt.Errorf("no valid IPv6 address found from server %s", *d)
}
