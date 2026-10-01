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
		slog.Debug("IPv6 address unchanged, skipping update", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}

	return syncDNSRecord(ctx, d, p, ipv6)
}

// syncDNSRecord 查询根域名记录并同步当前子域名（调用方须已持锁）。
func syncDNSRecord(ctx context.Context, d *Domain, p DNSProvider, addr net.IP) error {
	// 按根域名查询，便于同 zone 多子域复用（见 syncDomainGroup）
	records, err := p.GetRecords(ctx, d.Domain, d.Type)
	if err != nil {
		slog.Error("failed to query records", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"ipv6", addr.String(), "err", err)
		return fmt.Errorf("failed to query records: %w", err)
	}
	return applyDNSRecords(ctx, d, p, addr, records)
}

// applyDNSRecords 用已查询的 records 同步单个子域名（调用方须已持锁）。
//
// 工作流程：
//  1. 只处理匹配当前子域名且类型相符的记录
//  2. 同 IP 则跳过，不同 IP 则修改
//  3. 目标子域名下无匹配记录则新增
func applyDNSRecords(ctx context.Context, d *Domain, p DNSProvider, addr net.IP, records []RecordInfo) error {
	fqdn := d.FullDomain()
	ipv6Str := addr.String()

	slog.Debug("applying DNS records", "module", "ddns",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "type", d.Type, "record_count", len(records))

	found := false

	for _, r := range records {
		if !RecordNameMatches(r.Name, fqdn, d.SubDomain) || r.Type != d.Type {
			continue
		}
		found = true

		slog.Debug("comparing DNS record values", "module", "ddns",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"existing_value", r.Value, "new_value", ipv6Str,
			"record_id", r.ID, "record_type", r.Type)

		if ipv6Equal(addr, r.Value) {
			copyAddrToDomain(d, addr)
			slog.Debug("IPv6 record already matches, no update needed", "module", "ddns",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"record_id", r.ID)
			continue
		}

		err := p.ModifyRecord(ctx, RecordInfo{
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

		err := p.AddRecord(ctx, RecordInfo{
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
