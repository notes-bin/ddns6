package ipaddr

import (
	"context"
	"fmt"
	"log/slog"
	"net"
)

// DnsFetcher 通过向指定 DNS 服务器发起 UDP6 拨号，从本地套接字地址获取本机 IPv6。
//
// 底层值为 DNS 服务器 IPv6 地址（不含端口，固定使用 53）。
// 不依赖外部 HTTP，适合公网 HTTP 不可达但具备出站 IPv6 的环境。
type DnsFetcher string

// NewDnsFetcher 创建指向 server（IPv6 字面量）的 DNS 获取器。
func NewDnsFetcher(server string) *DnsFetcher {
	return (*DnsFetcher)(&server)
}

// String 返回 DNS 服务器地址。
func (d *DnsFetcher) String() string {
	return string(*d)
}

// Fetch 拨号 DNS 服务器并返回本地 UDP 地址中的全局 IPv6；失败则返回错误。
func (d *DnsFetcher) Fetch(ctx context.Context) (net.IP, error) {
	slog.Debug("fetching IPv6 via DNS", "module", "ipaddr", "dns_server", d.String())

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "udp6", fmt.Sprintf("[%s]:53", *d))
	if err != nil {
		return nil, fmt.Errorf("dial DNS server failed: %w", err)
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("unexpected local address type from dial")
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

	return nil, fmt.Errorf("no valid IPv6 address obtained from dial")
}
