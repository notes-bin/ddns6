package ipaddr

import (
	"testing"
)

// 测试 NewDNSProvider 函数
func TestNewDNSProvider(t *testing.T) {
	provider := NewDNSProvider()
	if provider == nil {
		t.Errorf("NewDNSProvider() 返回了 nil，期望一个有效的 *DNSProvider 实例")
	}
	if len(provider.servers) == 0 {
		t.Errorf("NewDNSProvider() 生成的 DNS 服务器列表为空，期望有服务器地址")
	}
}

// 测试 getIPv6FromServer 函数
func TestGetIPv6FromServer(t *testing.T) {
	// 这里使用一个示例 DNS 服务器地址
	server := "2400:3200:baba::1"
	ip, err := getIPv6FromServer(server)
	if err != nil {
		// 注意：由于网络环境等因素，此测试可能会失败
		t.Logf("从服务器 %s 获取 IPv6 地址时出错: %v", server, err)
	} else {
		if ip == nil {
			t.Errorf("getIPv6FromServer(%s) 返回了 nil IP，期望一个有效的 IPv6 地址", server)
		}
		if ip.To16() == nil {
			t.Errorf("getIPv6FromServer(%s) 返回的不是有效的 IPv6 地址", server)
		}
	}
}

// 测试 DNSProvider 的 GetIPv6Addr 方法
func TestDNSProvider_GetIPv6Addr(t *testing.T) {
	provider := NewDNSProvider()
	ip, err := provider.GetIPv6Addr()
	if err != nil {
		// 注意：由于网络环境等因素，此测试可能会失败
		t.Logf("从所有 DNS 服务器获取 IPv6 地址时出错: %v", err)
	} else {
		if ip == nil {
			t.Errorf("DNSProvider.GetIPv6Addr() 返回了 nil IP，期望一个有效的 IPv6 地址")
		}
		if ip.To16() == nil {
			t.Errorf("DNSProvider.GetIPv6Addr() 返回的不是有效的 IPv6 地址")
		}
	}
}
