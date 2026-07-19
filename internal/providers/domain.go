// Package providers 提供 DNS 服务商管理的通用工具和接口
package providers

import "strings"

// SplitDomain 将完整域名分割为根域名和子域名
// 假设根域名为最后两部分（如 example.com），前面的部分为子域名
// 例如：www.example.com -> (example.com, www), example.com -> (example.com, @)
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
