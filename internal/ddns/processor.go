// Package ddns 提供动态域名解析（DDNS）服务编排。
package ddns

import (
	"context"
)

// CollectMatchingRecords 查询 DNS 记录并收集匹配的记录。
//
// 模板方法：按根域名分组 → 逐组查询 → 去重 → 匹配子域名 → 收集结果。
//
// 参数:
//   - p: DNS 记录查询器
//   - domains: 域名配置列表（含子域名）
//   - recordType: 记录类型过滤（如 "AAAA"），空字符串表示不按类型过滤
//   - filterBySubdomain: 是否按子域名过滤。为 true 时只保留匹配指定子域名的记录；
//     为 false 时返回该根域名下所有指定类型的记录
//
// 去重规则：同一记录（ID+Name+Type+Value 相同）只保留第一条。
func CollectMatchingRecords(ctx context.Context, p DNSRecordGetter, domains []*Domain, recordType string, filterBySubdomain bool) ([]RecordInfo, error) {
	// 按根域名分组，每个根域名只查一次 API
	rootGroups := make(map[string][]*Domain)
	for _, d := range domains {
		rootGroups[d.Domain] = append(rootGroups[d.Domain], d)
	}

	var allRecords []RecordInfo
	seen := make(map[string]bool)

	for rootDomain, group := range rootGroups {
		records, err := p.GetRecords(ctx, rootDomain, recordType)
		if err != nil {
			return nil, err
		}

		for _, r := range records {
			// 按子域名过滤
			if filterBySubdomain {
				matched := false
				for _, d := range group {
					if RecordNameMatches(r.Name, d.FullDomain(), d.SubDomain) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}

			// 按记录类型过滤
			if recordType != "" && r.Type != recordType {
				continue
			}

			// 去重
			if seen[r.Key()] {
				continue
			}
			seen[r.Key()] = true
			allRecords = append(allRecords, r)
		}
	}

	return allRecords, nil
}
