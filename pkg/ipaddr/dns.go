package ipaddr

import (
	"context"
	"fmt"
	"log/slog"
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
	slog.Debug("fetching IPv6 via DNS", "module", "ipaddr", "dns_server", d.String())

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "udp6", fmt.Sprintf("[%s]:53", *d))
	if err != nil {
		return nil, fmt.Errorf("failed to dial DNS server %s: %w", *d, err)
	}
	defer conn.Close()

	// 获取本地地址
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("unexpected local address type for server %s", *d)
	}
	if localAddr.IP.To16() != nil && localAddr.IP.To4() == nil {
		slog.Info("got local IPv6 address via DNS",
			"module", "ipaddr",
			"dns_server", d.String(),
			"local_addr", localAddr.IP.String())
		return localAddr.IP, nil
	}

	slog.Warn("DNS dial did not return a valid IPv6 address",
		"module", "ipaddr",
		"dns_server", d.String(),
		"local_addr", localAddr.IP.String())

	return nil, fmt.Errorf("no valid IPv6 address found from server %s", *d)
}
