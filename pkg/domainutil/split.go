// Package domainutil 提供域名相关工具函数
package domainutil

import "strings"

// SplitDomain 将完整域名分割为根域名和子域名。
//
// rootDomain 参数为已知的根域名（来自 --domain 参数），当非空时直接用于分割，
// 能正确处理 example.co.uk 等多部分 TLD。rootDomain 为空时回退到旧逻辑
//（取最后两个点号分隔部分作为根域名）。
//
// 例如：
//
//	SplitDomain("www.example.com", "example.com")      -> ("example.com", "www")
//	SplitDomain("example.com", "example.com")           -> ("example.com", "@")
//	SplitDomain("sub.www.example.com", "example.com")  -> ("example.com", "sub.www")
//	SplitDomain("www.example.co.uk", "example.co.uk")  -> ("example.co.uk", "www")
func SplitDomain(fulldomain, rootDomain string) (root, subDomain string) {
	if rootDomain != "" {
		// 已知根域名，直接从完整域名中剥离
		if fulldomain == rootDomain {
			return rootDomain, "@"
		}
		if strings.HasSuffix(fulldomain, "."+rootDomain) && len(fulldomain) > len(rootDomain)+1 {
			subDomain = strings.TrimSuffix(fulldomain, "."+rootDomain)
			return rootDomain, subDomain
		}
		// 降级：fulldomain 可能不包含 rootDomain 后缀
	}

	// 旧逻辑：取最后两个点号分隔部分作为根域名（用于无 rootDomain 时向后兼容）
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
