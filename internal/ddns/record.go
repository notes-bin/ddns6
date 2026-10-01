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
// 仅在读写 Addr 缓存时短暂持锁，DNS API I/O 在锁外执行。
// 失败不在此记 Error（由 syncDomainGroup 等边界统一记录，避免叠层）。
//
// 参数:
//   - ctx: 取消时中止操作；可携带 sync_id
//   - d: 域名配置（含子域名、记录类型等）
//   - ipv6: 当前本机 IPv6 地址
//   - p: DNS 服务商实现
func SyncRecord(ctx context.Context, d *Domain, ipv6 net.IP, p DNSProvider) error {
	select {
	case <-ctx.Done():
		slog.InfoContext(ctx, "sync task cancelled", "module", "ddns",
			"sync_id", SyncIDFrom(ctx), "domain", d.Domain, "subdomain", d.SubDomain)
		return ctx.Err()
	default:
	}

	d.lock()
	unchanged := !hasAddressChanged(d.Addr, ipv6)
	d.unlock()
	if unchanged {
		slog.DebugContext(ctx, "IPv6 address unchanged, skipping update", "module", "ddns",
			"sync_id", SyncIDFrom(ctx), "domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}

	return syncDNSRecord(ctx, d, p, ipv6)
}

// syncDNSRecord 查询根域名记录并同步当前子域名（不持 Domain 锁）。
func syncDNSRecord(ctx context.Context, d *Domain, p DNSProvider, addr net.IP) error {
	records, err := p.GetRecords(ctx, d.Domain, d.Type)
	if err != nil {
		return fmt.Errorf("failed to query records for %s/%s: %w", d.Domain, d.SubDomain, err)
	}
	return applyDNSRecords(ctx, d, p, addr, records)
}

// applyDNSRecords 用已查询的 records 同步单个子域名。
//
// DNS API 调用在锁外执行；仅更新 Addr 缓存时短暂加锁。
// Domain/SubDomain/Type/TTL 在服务启动后视为只读。
// 失败只返回 error，由调用方边界记一次日志。
func applyDNSRecords(ctx context.Context, d *Domain, p DNSProvider, addr net.IP, records []RecordInfo) error {
	fqdn := d.FullDomain()
	ipv6Str := addr.String()
	syncID := SyncIDFrom(ctx)
	ttl := RecordTTL(d.TTL)

	slog.DebugContext(ctx, "applying DNS records", "module", "ddns",
		"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "record_type", d.Type, "record_count", len(records))

	found := false

	for _, r := range records {
		if !RecordNameMatches(r.Name, fqdn, d.SubDomain) || r.Type != d.Type {
			continue
		}
		found = true

		slog.DebugContext(ctx, "comparing DNS record values", "module", "ddns",
			"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain,
			"existing_value", r.Value, "new_value", ipv6Str,
			"record_id", r.ID, "record_type", r.Type)

		if ipv6Equal(addr, r.Value) {
			copyAddrToDomain(d, addr)
			slog.DebugContext(ctx, "IPv6 record already matches, no update needed", "module", "ddns",
				"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain,
				"record_id", r.ID)
			continue
		}

		err := p.ModifyRecord(ctx, RecordInfo{
			ID: r.ID, Name: fqdn, Zone: d.Domain, Type: d.Type, Value: ipv6Str, TTL: ttl,
		})
		if err != nil {
			return fmt.Errorf("failed to modify record %s/%s: %w", d.Domain, d.SubDomain, err)
		}
		copyAddrToDomain(d, addr)
		slog.InfoContext(ctx, "IPv6 address changed, record modified", "module", "ddns",
			"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain,
			"ipv6", ipv6Str, "record_id", r.ID)
	}

	if !found {
		slog.DebugContext(ctx, "no AAAA record found, adding new record", "module", "ddns",
			"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain,
			"fqdn", fqdn, "ipv6", ipv6Str)

		err := p.AddRecord(ctx, RecordInfo{
			Name: fqdn, Zone: d.Domain, Type: d.Type, Value: ipv6Str, TTL: ttl,
		})
		if err != nil {
			return fmt.Errorf("failed to add record %s/%s: %w", d.Domain, d.SubDomain, err)
		}
		copyAddrToDomain(d, addr)
		slog.InfoContext(ctx, "IPv6 address changed, record added", "module", "ddns",
			"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str)
	}

	return nil
}

// copyAddrToDomain 将 IP 拷贝到 Domain.Addr（内部加锁）。
func copyAddrToDomain(d *Domain, addr net.IP) {
	d.lock()
	defer d.unlock()
	d.Addr = make(net.IP, len(addr))
	copy(d.Addr, addr)
}
