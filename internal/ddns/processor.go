package ddns

import (
	"context"
	"fmt"
)

// CollectMatchingRecords 查询 DNS 记录并收集匹配结果，供 records/clean 使用。
//
// 按根域名分组（每组只查一次 GetRecords，与同步路径的 root-domain merge 一致）
// -> 匹配子域名 -> 去重 -> 汇总。
//
// 参数:
//   - p: DNS 记录查询器
//   - domains: 域名配置列表（含子域名）
//   - recordType: 记录类型过滤（如 "AAAA"），空字符串表示不按类型过滤
//   - filterBySubdomain: 为 true 时只保留匹配指定子域名的记录；
//     为 false 时返回该根域名下所有指定类型的记录
//
// 去重规则：同一记录（ID+Name+Type+Value 相同）只保留第一条。
func CollectMatchingRecords(ctx context.Context, p DNSProvider, domains []*Domain, recordType string, filterBySubdomain bool) ([]RecordInfo, error) {
	rootGroups := make(map[string][]*Domain, len(domains))
	for _, d := range domains {
		rootGroups[d.Domain] = append(rootGroups[d.Domain], d)
	}

	allRecords := make([]RecordInfo, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))

	for rootDomain, group := range rootGroups {
		records, err := p.GetRecords(ctx, rootDomain, recordType)
		if err != nil {
			return nil, fmt.Errorf("query records for %s: %w", rootDomain, err)
		}

		// 预计算 FQDN，避免内层循环反复 FullDomain 字符串拼接
		type matchTarget struct {
			fqdn string
			sub  string
		}
		var targets []matchTarget
		if filterBySubdomain {
			targets = make([]matchTarget, len(group))
			for i, d := range group {
				targets[i] = matchTarget{fqdn: d.FullDomain(), sub: d.SubDomain}
			}
		}

		for _, r := range records {
			if filterBySubdomain {
				matched := false
				for _, t := range targets {
					if RecordNameMatches(r.Name, t.fqdn, t.sub) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}

			if recordType != "" && r.Type != recordType {
				continue
			}

			key := r.Key()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			allRecords = append(allRecords, r)
		}
	}

	return allRecords, nil
}
