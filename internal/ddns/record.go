// Package ddns 提供动态域名解析（DDNS）服务编排。
//
// 核心流程：
//
//	Linux: Netlink 监听地址变化 → debounce 10s → 获取 IPv6 → 同步 DNS 记录
//	其他:   cron 定时轮询 → 获取 IPv6 → 同步 DNS 记录
//
// 同步策略：
//  1. 查询目标子域名的所有 AAAA 记录
//  2. 遍历匹配的子域名记录，IP 相同则跳过，不同则修改
//  3. 目标子域名无 AAAA 记录则新增
//  4. 同一个子域名下有多个 AAAA 记录则全部处理
package ddns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
)

// SyncRecord 同步 DNS 记录与当前 IPv6 地址一致。
//
// 先检查地址是否变化，若无变化则跳过更新。变化则调用 syncDNSRecord 执行同步。
// 使用 d.Lock()/d.Unlock() 保护 Domain 的并发访问。
//
// 参数:
//   - ctx: 上下文，取消时中止操作
//   - d: 域名配置（含子域名、记录类型等）
//   - ipv6: 当前本机 IPv6 地址
//   - p: DNS 服务商实现
func SyncRecord(ctx context.Context, d *Domain, ipv6 net.IP, p DNSProvider) error {
	d.Lock()
	defer d.Unlock()

	// 检查 context 是否已取消
	select {
	case <-ctx.Done():
		slog.Info("sync task cancelled", "module", "ddns", "domain", d.Domain, "subdomain", d.SubDomain)
		return ctx.Err()
	default:
	}

	// 地址未变化则跳过，避免无效的 API 调用
	if !hasAddressChanged(d.Addr, ipv6) {
		slog.Info("IPv6 address unchanged, skipping update", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}

	return syncDNSRecord(ctx, d, p, ipv6)
}

// syncDNSProvider 是 DNS 记录同步所需的操作子集。
//
// syncDNSRecord 只需要查询、新增和修改能力，无需删除能力。
// 通过缩小接口依赖，使测试和后续扩展更加灵活。
type syncDNSProvider interface {
	DNSRecordGetter
	DNSRecordAdder
	DNSRecordModifier
}

// syncDNSRecord 执行实际的 DNS 记录同步。
//
// 工作流程：
//  1. 通过 p.GetRecords() 查询目标子域名下所有记录
//  2. 遍历记录，只处理匹配当前子域名且类型为 AAAA 的记录
//  3. 同 IP 则跳过，不同 IP 则修改
//  4. 目标子域名下无 AAAA 记录则新增
//
// 同一个子域名下存在多个 AAAA 记录时全部处理（continue 而非 return）。
func syncDNSRecord(ctx context.Context, d *Domain, p syncDNSProvider, addr net.IP) error {
	fqdn := d.FullDomain()
	ipv6Str := addr.String()

	slog.Debug("querying existing DNS records", "module", "ddns",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "type", d.Type)

	// 查询当前 DNS 记录（各服务商在 API 层可能已按 fqdn 过滤，
	// 但有些服务商会返回整个 zone 的记录）
	records, err := p.GetRecords(ctx, fqdn, d.Type)
	if err != nil {
		slog.Error("failed to query records", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"ipv6", ipv6Str, "err", err)
		return fmt.Errorf("failed to query records: %w", err)
	}

	slog.Debug("DNS records query completed", "module", "ddns",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"record_count", len(records))

	found := false // 是否找到匹配的子域名 AAAA 记录

	for _, r := range records {
		// 过滤：只处理匹配目标子域名 + 目标类型的记录。
		// RecordNameMatches 处理不同服务商返回的记录名格式差异。
		if !RecordNameMatches(r.Name, fqdn, d.SubDomain) || r.Type != d.Type {
			continue
		}
		found = true

		slog.Debug("comparing DNS record values", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"existing_value", r.Value, "new_value", ipv6Str,
			"record_id", r.ID, "record_type", r.Type)

		// IP 相同则更新缓存后跳过（多个 AAAA 记录时继续处理下一条）
		if ipv6Equal(r.Value, ipv6Str) {
			d.SetAddr(addr)
			slog.Debug("IPv6 record already matches, no update needed", "module", "ddns",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"record_id", r.ID)
			continue
		}

		// IP 不同 → 修改记录
		err = p.ModifyRecord(ctx, RecordInfo{
			ID: r.ID, Name: fqdn, Zone: d.Domain, Type: d.Type, Value: ipv6Str, TTL: d.TTL,
		})
		if err != nil {
			slog.Error("failed to modify record", "module", "ddns",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"ipv6", ipv6Str, "record_id", r.ID, "err", err)
			return fmt.Errorf("failed to modify record: %w", err)
		}
		d.SetAddr(addr)
		slog.Info("IPv6 address changed, record modified", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"ipv6", ipv6Str, "record_id", r.ID)
	}

	// 目标子域名下无 AAAA 记录 → 新增
	if !found {
		slog.Debug("no AAAA record found, adding new record", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"fqdn", fqdn, "ipv6", ipv6Str)

		err = p.AddRecord(ctx, RecordInfo{
			Name: fqdn, Zone: d.Domain, Type: d.Type, Value: ipv6Str, TTL: d.TTL,
		})
		if err != nil {
			slog.Error("failed to add record", "module", "ddns",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"ipv6", ipv6Str, "err", err)
			return fmt.Errorf("failed to add record: %w", err)
		}
		d.SetAddr(addr)
		slog.Info("IPv6 address changed, record added", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str)
	}

	return nil
}
