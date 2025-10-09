package ipaddr

import (
	"fmt"
	"net"
)

// DNSProvider 从 DNS 服务器获取 IPv6 地址
type DNSProvider struct {
	servers []string `env:"IPv6_DNS"`
}

// NewDNSProvider 创建一个新的 DNSProvider
func NewDNSProvider() *DNSProvider {
	return &DNSProvider{
		servers: []string{"2400:3200:baba::1", "2606:4700:4700::1111", "2001:4860:4860::8888"},
	}
}

// getIPv6FromServer 尝试从指定 DNS 服务器获取 IPv6 地址
func getIPv6FromServer(server string) (net.IP, error) {
	conn, err := net.Dial("udp6", fmt.Sprintf("[%s]:53", server))
	if err != nil {
		return nil, fmt.Errorf("failed to dial DNS server %s: %w", server, err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	if localAddr.IP.To16() != nil {
		return localAddr.IP, nil
	}
	return nil, fmt.Errorf("no valid IPv6 address found from server %s", server)
}

// GetIPv6Addr 获取 DNS 服务器的 IPv6 地址
func (d *DNSProvider) GetIPv6Addr() (net.IP, error) {
	for _, server := range d.servers {
		ip, err := getIPv6FromServer(server)
		if err == nil {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("failed to get IPv6 address from all DNS servers")
}
