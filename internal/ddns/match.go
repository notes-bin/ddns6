package ddns

import (
	"log/slog"
	"net"
	"strings"
)

// hasAddressChanged 判断缓存地址与新地址是否不同（缓存为 nil 视为已变化）。
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

// ipv6Equal 比较已解析 IP 与字符串形式的地址是否相等。
//
// 当 b 无法解析为有效 IP 时，回退到 a.String() 与 b 的字符串比较。
func ipv6Equal(a net.IP, b string) bool {
	ipB := net.ParseIP(b)
	if ipB == nil {
		return a.String() == b
	}
	if a == nil {
		return false
	}
	return a.Equal(ipB)
}

// RecordNameMatches 判断 DNS 记录名是否匹配目标子域名。
//
// 不同服务商返回的记录名格式不一致，需兼容：
//   - 完整域名（www.example.com）
//   - 完整域名后带点号（www.example.com.）
//   - 仅子域名标签（www、@）
//   - 根域名返回空字符串或 "@"
//
// 参数:
//   - rName: 服务商返回的记录名
//   - fqdn: 完整目标子域名（FullDomain 的返回值）
//   - subDomain: 子域名标签（Domain.SubDomain）
func RecordNameMatches(rName, fqdn, subDomain string) bool {
	// 去掉尾部点号（部分服务商如 HuaweiCloud 返回 FQDN.）
	name := strings.TrimSuffix(rName, ".")

	// Cloudflare 等返回完整域名
	if name == fqdn {
		return true
	}

	// Tencent、AliCloud 等只返回主机标签
	if name == subDomain {
		return true
	}

	// 根域名：部分服务商返回 "" 或 "@"
	if subDomain == "@" {
		if name == "" || name == "@" {
			return true
		}
	}

	return false
}
