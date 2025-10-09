package ipaddr

import (
	"testing"
)

// TestNewSiteProvider 测试 NewSiteProvider 函数是否能正确创建 SiteProvider 实例
func TestNewSiteProvider(t *testing.T) {
	provider := NewSiteProvider()
	if provider == nil {
		t.Errorf("NewSiteProvider() 返回了 nil，期望一个有效的 *SiteProvider 实例")
	}
	if len(provider.urls) == 0 {
		t.Errorf("NewSiteProvider() 生成的 URL 列表为空，期望有 URL 地址")
	}
}

// TestGetIPv6FromURL 测试 getIPv6FromURL 函数能否从指定 URL 获取有效的 IPv6 地址
func TestGetIPv6FromURL(t *testing.T) {
	// 取一个示例 URL
	url := "http://ipv6.icanhazip.com/"
	ip, err := getIPv6FromURL(url)
	if err != nil {
		// 注意：由于网络环境等因素，此测试可能会失败
		t.Logf("从 URL %s 获取 IPv6 地址时出错: %v", url, err)
	} else {
		if ip == nil {
			t.Errorf("getIPv6FromURL(%s) 返回了 nil IP，期望一个有效的 IPv6 地址", url)
		}
		if ip.To16() == nil {
			t.Errorf("getIPv6FromURL(%s) 返回的不是有效的 IPv6 地址", url)
		}
	}
}

// TestSiteProvider_GetIPv6Addr 测试 SiteProvider 的 GetIPv6Addr 方法能否从所有 URL 中获取有效的 IPv6 地址
func TestSiteProvider_GetIPv6Addr(t *testing.T) {
	provider := NewSiteProvider()
	ip, err := provider.GetIPv6Addr()
	if err != nil {
		// 注意：由于网络环境等因素，此测试可能会失败
		t.Logf("从所有网站获取 IPv6 地址时出错: %v", err)
	} else {
		if ip == nil {
			t.Errorf("SiteProvider.GetIPv6Addr() 返回了 nil IP，期望一个有效的 IPv6 地址")
		}
		if ip.To16() == nil {
			t.Errorf("SiteProvider.GetIPv6Addr() 返回的不是有效的 IPv6 地址")
		}
	}
}
