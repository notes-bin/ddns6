package iputil

import (
	"testing"
)

// TestNewIfaceProvider 测试 NewIfaceProvider 函数
func TestNewIfaceProvider(t *testing.T) {
	provider := NewIfaceProvider()
	if provider == nil {
		t.Errorf("NewIfaceProvider() 返回了 nil，期望一个有效的 *IfaceProvider 实例")
	}
	if provider.name != "eth0" {
		t.Errorf("NewIfaceProvider() 创建的实例的默认网卡名称不是 eth0，实际为 %s", provider.name)
	}
}

// TestIfaceProvider_GetIPv6Addr 测试 IfaceProvider 的 GetIPv6Addr 方法
func TestIfaceProvider_GetIPv6Addr(t *testing.T) {
	provider := NewIfaceProvider()
	ip, err := provider.GetIPv6Addr()
	if err != nil {
		// 打印错误信息，但不直接断言失败，因为可能当前环境确实没有可用的 IPv6 地址
		t.Logf("从网卡 %s 获取 IPv6 地址时出错: %v", provider.name, err)
	} else {
		if ip == nil {
			t.Errorf("IfaceProvider.GetIPv6Addr() 返回了 nil IP，期望一个有效的 IPv6 地址")
		}
		if ip.To4() != nil {
			t.Errorf("IfaceProvider.GetIPv6Addr() 返回的不是 IPv6 地址，而是 IPv4 地址 %s", ip)
		}
		if !ip.IsGlobalUnicast() {
			t.Errorf("IfaceProvider.GetIPv6Addr() 返回的不是全局单播 IPv6 地址 %s", ip)
		}
	}
}
