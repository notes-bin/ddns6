package ipaddr

import (
	"fmt"
	"net"
)

// IfaceProvider 从网卡获取 IPv6 地址
type IfaceProvider struct {
	name string `env:"IPv6_IFACE" default:"eth0"`
}

// NewIfaceProvider 创建一个新的 IfaceProvider
func NewIfaceProvider() *IfaceProvider {
	return &IfaceProvider{name: "eth0"} // 默认网卡名称
}

// GetIPv6Addr 获取网卡的 IPv6 地址
func (i *IfaceProvider) GetIPv6Addr() (net.IP, error) {
	iface, err := net.InterfaceByName(i.name)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface %s: %w", i.name, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %s: %w", i.name, err)
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipNet.IP
		if ip.To4() == nil && ip.IsGlobalUnicast() {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no IPv6 address found on interface %s", i.name)
}
