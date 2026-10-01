// Package domainutil 提供域名拆分与 zone 候选生成工具。
//
// SplitDomain 将完整 FQDN 拆为根域名与子域名，供 DDNS 同步与各 DNS 运营商 API 使用；
// ZoneCandidates 生成从长到短的 zone 查找候选列表。
package domainutil

import "strings"

// SplitDomain 将完整域名分割为根域名和子域名。
//
// rootDomain 为已知根域名（通常来自 --domain）。非空时按后缀剥离子域名，
// 可正确处理 example.co.uk 等多部分 TLD；为空时回退为取最后两段作为根域名
// （兼容旧调用方，多部分 TLD 在此路径下不准确）。
//
// 返回值约定：apex（主机名即根域名）时 subDomain 为 "@"。
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
		if subDomain, ok := strings.CutSuffix(fulldomain, "."+rootDomain); ok && subDomain != "" {
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

// ZoneCandidates 返回按从长到短排列的 zone 查找候选列表。
//
// 处理规则：
//  1. 先转小写并去掉尾部 "."；
//  2. 规范化后为空则返回 nil；
//  3. 单标签（无 "."）返回仅含自身的切片；
//  4. 多标签时返回所有至少含两段的后缀（含完整域名本身），
//     不含最右侧的单段纯 TLD（如 com、uk）。
//
// 本函数不做 Public Suffix List（公共后缀列表）判断。
// 因此 ZoneCandidates("www.example.co.uk") 会包含 "co.uk"（两段），
// 调用方需在运营商 API 侧匹配真实 zone。
//
// 例如：
//
//	ZoneCandidates("www.example.com")  → ["www.example.com", "example.com"]
//	ZoneCandidates("example.com")       → ["example.com"]
//	ZoneCandidates("a.b.example.com")   → ["a.b.example.com", "b.example.com", "example.com"]
//	ZoneCandidates("www.example.co.uk") → ["www.example.co.uk", "example.co.uk", "co.uk"]
//	ZoneCandidates("localhost")         → ["localhost"]
//	ZoneCandidates("") / ZoneCandidates(".") → nil
func ZoneCandidates(fulldomain string) []string {
	candidate := strings.ToLower(strings.TrimSuffix(fulldomain, "."))
	if candidate == "" {
		return nil
	}
	parts := strings.Split(candidate, ".")
	if len(parts) < 2 {
		return []string{candidate}
	}
	out := make([]string, 0, len(parts)-1)
	for i := range len(parts) - 1 {
		out = append(out, strings.Join(parts[i:], "."))
	}
	return out
}
