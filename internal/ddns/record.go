package ddns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
)

// SyncRecord 将 DNS 记录同步为当前 IPv6 地址。
//
// 地址未变化则跳过；变化则调用 syncDNSRecord。
// 通过 d.lock()/d.unlock() 保护 Domain 的并发访问。
//
// 参数:
//   - ctx: 取消时中止操作
//   - d: 域名配置（含子域名、记录类型等）
//   - ipv6: 当前本机 IPv6 地址
//   - p: DNS 服务商实现
func SyncRecord(ctx context.Context, d *Domain, ipv6 net.IP, p DNSProvider) error {
	d.lock()
	defer d.unlock()

	select {
	case <-ctx.Done():
		slog.Info("sync task cancelled", "module", "ddns", "domain", d.Domain, "subdomain", d.SubDomain)
		return ctx.Err()
	default:
	}

	// 未变化则跳过，避免无效 API 调用
	if !hasAddressChanged(d.Addr, ipv6) {
		slog.Info("IPv6 address unchanged, skipping update", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}

	return syncDNSRecord(ctx, d, p, ipv6)
}

// syncDNSRecord 执行实际的 DNS 记录同步。
//
// 工作流程：
//  1. 通过 p.GetRecords 查询记录
//  2. 只处理匹配当前子域名且类型相符的记录
//  3. 同 IP 则跳过，不同 IP 则修改
//  4. 目标子域名下无匹配记录则新增
//
// 同一子域名存在多条匹配记录时全部处理（continue 而非 return）。
func syncDNSRecord(ctx context.Context, d *Domain, p DNSProvider, addr net.IP) error {
	fqdn := d.FullDomain()
	ipv6Str := addr.String()

	slog.Debug("querying existing DNS records", "module", "ddns",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "type", d.Type)

	// 部分服务商已在 API 层按 fqdn 过滤，另一些会返回整个 zone
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

	found := false

	for _, r := range records {
		// RecordNameMatches 抹平各服务商记录名格式差异
		if !RecordNameMatches(r.Name, fqdn, d.SubDomain) || r.Type != d.Type {
			continue
		}
		found = true

		slog.Debug("comparing DNS record values", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"existing_value", r.Value, "new_value", ipv6Str,
			"record_id", r.ID, "record_type", r.Type)

		// IP 相同：仍更新本地缓存，并继续处理同名其他记录
		if ipv6Equal(addr, r.Value) {
			copyAddrToDomain(d, addr)
			slog.Debug("IPv6 record already matches, no update needed", "module", "ddns",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"record_id", r.ID)
			continue
		}

		err = p.ModifyRecord(ctx, RecordInfo{
			ID: r.ID, Name: fqdn, Zone: d.Domain, Type: d.Type, Value: ipv6Str, TTL: d.TTL,
		})
		if err != nil {
			slog.Error("failed to modify record", "module", "ddns",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"ipv6", ipv6Str, "record_id", r.ID, "err", err)
			return fmt.Errorf("failed to modify record: %w", err)
		}
		copyAddrToDomain(d, addr)
		slog.Info("IPv6 address changed, record modified", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"ipv6", ipv6Str, "record_id", r.ID)
	}

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
		copyAddrToDomain(d, addr)
		slog.Info("IPv6 address changed, record added", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str)
	}

	return nil
}

// copyAddrToDomain 将 IP 拷贝到 Domain.Addr（调用方必须已持有 d 的锁）。
func copyAddrToDomain(d *Domain, addr net.IP) {
	d.Addr = make(net.IP, len(addr))
	copy(d.Addr, addr)
}
