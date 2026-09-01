// Package domainutil 提供域名拆分工具。
//
// 主要入口为 SplitDomain：将完整 FQDN 拆为根域名与子域名，
// 供 DDNS 同步与各 DNS 运营商 API 使用。
package domainutil

import "strings"

// SplitDomain 将完整域名分割为根域名和子域名。
//
// rootDomain 为已知根域名（通常来自 --domain）；非空时按后缀剥离子域名，
// 可正确处理 example.co.uk 等多部分 TLD。为空时回退为取最后两段作为根域名
// （兼容旧调用方）。
//
// 例如：
//
//	SplitDomain("www.example.com", "example.com")      -> ("example.com", "www")
//	SplitDomain("example.com", "example.com")           -> ("example.com", "@")
//	SplitDomain("sub.www.example.com", "example.com")  -> ("example.com", "sub.www")
//	SplitDomain("www.example.co.uk", "example.co.uk")  -> ("example.co.uk", "www")
func SplitDomain(fulldomain, rootDomain string) (root, subDomain string) {
	if rootDomain != "" {
		if fulldomain == rootDomain {
			return rootDomain, "@"
		}
		if strings.HasSuffix(fulldomain, "."+rootDomain) && len(fulldomain) > len(rootDomain)+1 {
			subDomain = strings.TrimSuffix(fulldomain, "."+rootDomain)
			return rootDomain, subDomain
		}
		// fulldomain 不含 rootDomain 后缀时降级到下方旧逻辑
	}

	// 无已知根域名：取最后两段为根（多部分 TLD 在此路径下不准确）
	parts := strings.Split(fulldomain, ".")
	if len(parts) < 2 {
		return fulldomain, "@"
	}
	root = strings.Join(parts[len(parts)-2:], ".")
	if len(parts) == 2 {
		return root, "@"
	}
	subDomain = strings.Join(parts[:len(parts)-2], ".")
	return root, subDomain
}
