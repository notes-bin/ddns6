package ddns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
)

// SyncRecord 将 DNS 记录同步为当前 IPv6 地址。
//
// 地址未变化则跳过；变化则调用 syncDNSRecord。
// 仅在读写 addr 缓存时短暂持锁，DNS API I/O 在锁外执行。
// 失败不在此记 Error（由 syncDomainGroup 等边界统一记录，避免叠层）。
func SyncRecord(ctx context.Context, d *Domain, ipv6 net.IP, p DNSProvider) error {
	select {
	case <-ctx.Done():
		slog.InfoContext(ctx, "sync task cancelled", "module", "ddns",
			"sync_id", SyncIDFrom(ctx), "domain", d.Domain, "subdomain", d.SubDomain)
		return ctx.Err()
	default:
	}

	d.lock()
	unchanged := !hasAddressChanged(d.addr, ipv6)
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
// DNS API 调用在锁外执行；仅当该子域名全部匹配记录处理成功后才更新 addr 缓存，
// 避免部分 Modify 成功后写缓存导致下一轮永久跳过、残留旧 AAAA。
func applyDNSRecords(ctx context.Context, d *Domain, p DNSProvider, addr net.IP, records []RecordInfo) error {
	fqdn := d.FullDomain()
	ipv6Str := addr.String()
	syncID := SyncIDFrom(ctx)
	ttl := RecordTTL(d.TTL)

	slog.DebugContext(ctx, "applying DNS records", "module", "ddns",
		"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "record_type", d.Type, "record_count", len(records))

	matched := make([]RecordInfo, 0)
	for _, r := range records {
		if RecordNameMatches(r.Name, fqdn, d.SubDomain) && r.Type == d.Type {
			matched = append(matched, r)
		}
	}

	if len(matched) == 0 {
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
		return nil
	}

	for _, r := range matched {
		slog.DebugContext(ctx, "comparing DNS record values", "module", "ddns",
			"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain,
			"existing_value", r.Value, "new_value", ipv6Str,
			"record_id", r.ID, "record_type", r.Type)

		if ipv6Equal(addr, r.Value) {
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
		slog.InfoContext(ctx, "IPv6 address changed, record modified", "module", "ddns",
			"sync_id", syncID, "domain", d.Domain, "subdomain", d.SubDomain,
			"ipv6", ipv6Str, "record_id", r.ID)
	}

	copyAddrToDomain(d, addr)
	return nil
}

// copyAddrToDomain 将 IP 拷贝到 Domain.addr（内部加锁）。
func copyAddrToDomain(d *Domain, addr net.IP) {
	d.lock()
	defer d.unlock()
	d.addr = slices.Clone(addr)
}

// hasAddressChanged 判断缓存地址与新地址是否不同（缓存为 nil 视为已变化）。
func hasAddressChanged(cached net.IP, newAddr net.IP) bool {
	changed := cached == nil || !cached.Equal(newAddr)
	if cached == nil {
		slog.Debug("no cached address, update needed", "module", "ddns")
	} else if changed {
		slog.Debug("IPv6 address has changed", "module", "ddns",
			"old_addr", cached.String(), "new_addr", newAddr.String())
	}
	return changed
}

// ipv6Equal 比较已解析 IP 与字符串形式的地址是否相等。
//
// 当 b 无法解析为有效 IP 时，回退到 a.String() 与 b 的字符串比较。
func ipv6Equal(a net.IP, b string) bool {
	ipB := net.ParseIP(b)
	if ipB == nil {
		return a.String() == b
	}
	if a == nil {
		return false
	}
	return a.Equal(ipB)
}

// RecordNameMatches 判断 DNS 记录名是否匹配目标子域名。
//
// 不同服务商返回的记录名格式不一致，需兼容：
//   - 完整域名（www.example.com）
//   - 完整域名后带点号（www.example.com.）
//   - 仅子域名标签（www、@）
//   - 根域名返回空字符串或 "@"
func RecordNameMatches(rName, fqdn, subDomain string) bool {
	name := strings.TrimSuffix(rName, ".")

	if name == fqdn {
		return true
	}
	if name == subDomain {
		return true
	}
	if subDomain == "@" {
		if name == "" || name == "@" {
			return true
		}
	}
	return false
}
