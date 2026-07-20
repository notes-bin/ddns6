// Package domainutil 提供域名相关工具函数
package domainutil

import "strings"

// SplitDomain 将完整域名分割为根域名和子域名
// 假设根域名为最后两部分（如 example.com），前面的部分为子域名
//
// 例如：
//
//	SplitDomain("www.example.com")   -> ("example.com", "www")
//	SplitDomain("example.com")       -> ("example.com", "@")
//	SplitDomain("sub.www.example.com") -> ("example.com", "sub.www")
func SplitDomain(fulldomain string) (root, subDomain string) {
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
