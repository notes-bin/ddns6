// Package ddns 提供动态域名解析（DDNS）服务编排。
package ddns

import (
	"log/slog"
	"net"
	"strings"
)

// hasAddressChanged 检查 IPv6 地址是否改变
func hasAddressChanged(cached net.IP, newAddr net.IP) bool {
	changed := cached == nil || !cached.Equal(newAddr)
	if cached == nil {
		slog.Debug("no cached address, update needed", "module", "ddns")
	} else if changed {
		slog.Debug("IPv6 address has changed", "module", "ddns",
			"old_addr", cached.String(), "new_addr", newAddr.String())
	}
	return changed
}

// ipv6Equal 归一化比较两个 IPv6 地址字符串是否相等
func ipv6Equal(a, b string) bool {
	ipA := net.ParseIP(a)
	ipB := net.ParseIP(b)
	if ipA == nil || ipB == nil {
		return a == b // 解析失败回退到字符串比较
	}
	return ipA.Equal(ipB)
}

// RecordNameMatches 判断 DNS 记录名是否匹配目标子域名。
//
// 不同服务商 API 返回的记录名格式不一致，需统一处理以下格式：
//   - 完整域名（www.example.com）
//   - 完整域名后带点号（www.example.com.）
//   - 仅子域名标签（www、@）
//   - 根域名返回空字符串或 "@"
//
// 参数:
//   - rName: 服务商返回的记录名
//   - fqdn: 完整目标子域名（FullDomain() 的返回值）
//   - subDomain: 子域名标签（Domain.SubDomain 字段值）
func RecordNameMatches(rName, fqdn, subDomain string) bool {
	// 标准化记录名：去除尾部点号（HuaweiCloud 等服务商返回带点号的域名）
	name := strings.TrimSuffix(rName, ".")

	// 1. 匹配完整域名（Cloudflare 等返回完整域名）
	if name == fqdn {
		return true
	}

	// 2. 匹配子域名标签（Tencent、AliCloud 等只返回标签）
	if name == subDomain {
		return true
	}

	// 3. 根域名特殊处理（子域名为 @ 时）
	if subDomain == "@" {
		// 某些服务商对根域名返回空字符串
		if name == "" {
			return true
		}
		// 某些服务商对根域名返回 "@" 标签
		if name == "@" {
			return true
		}
	}

	return false
}
