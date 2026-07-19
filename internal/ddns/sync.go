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
	"net"
	"strings"

	"github.com/notes-bin/ddns6/internal/providers"
)

// hasAddressChanged 检查 IPv6 地址是否改变
func hasAddressChanged(cached net.IP, newAddr net.IP) bool {
	changed := cached == nil || !cached.Equal(newAddr)
	if cached == nil {
		log.Debug("no cached address, update needed")
	} else if changed {
		log.Debug("IPv6 address has changed",
			"old_addr", cached.String(), "new_addr", newAddr.String())
	}
	return changed
}

// ipv6Equal 归一化比较两个 IPv6 地址字符串是否相等
func ipv6Equal(a, b string) bool {
	ipA := net.ParseIP(a)
	ipB := net.ParseIP(b)
	if ipA == nil || ipB == nil {
		return a == b // 解析失败回退到字符串比较
	}
	return ipA.Equal(ipB)
}

// recordNameMatches 判断 DNS 记录名是否匹配目标子域名。
//
// 不同服务商 API 返回的记录名格式不一致，需统一处理以下格式：
//   - 完整域名（www.example.com）
//   - 完整域名后带点号（www.example.com.）
//   - 仅子域名标签（www、@）
//   - 根域名返回空字符串或 "@"
//
// 参数:
//   - rName: 服务商返回的记录名
//   - fqdn: 完整目标子域名（FullDomain() 的返回值）
//   - subDomain: 子域名标签（Domain.SubDomain 字段值）
func recordNameMatches(rName, fqdn, subDomain string) bool {
	// 标准化记录名：去除尾部点号（HuaweiCloud 等服务商返回带点号的域名）
	name := strings.TrimSuffix(rName, ".")

	// 1. 匹配完整域名（Cloudflare 等返回完整域名）
	if name == fqdn {
		return true
	}

	// 2. 匹配子域名标签（Tencent、AliCloud 等只返回标签）
	if name == subDomain {
		return true
	}

	// 3. 根域名特殊处理（子域名为 @ 时）
	if subDomain == "@" {
		// 某些服务商对根域名返回空字符串
		if name == "" {
			return true
		}
		// 某些服务商对根域名返回 "@" 标签
		if name == "@" {
			return true
		}
	}

	return false
}

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
func SyncRecord(ctx context.Context, d *providers.Domain, ipv6 net.IP, p providers.DNSProvider) error {
	d.Lock()
	defer d.Unlock()

	// 检查 context 是否已取消
	select {
	case <-ctx.Done():
		log.Info("sync task cancelled", "domain", d.Domain, "subdomain", d.SubDomain)
		return ctx.Err()
	default:
	}

	// 地址未变化则跳过，避免无效的 API 调用
	if !hasAddressChanged(d.Addr, ipv6) {
		log.Info("IPv6 address unchanged, skipping update",
			"domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}

	return syncDNSRecord(ctx, d, p, ipv6)
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
func syncDNSRecord(ctx context.Context, d *providers.Domain, p providers.DNSProvider, addr net.IP) error {
	fqdn := d.FullDomain()
	ipv6Str := addr.String()

	log.Debug("querying existing DNS records",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "type", d.Type)

	// 查询当前 DNS 记录（各服务商在 API 层可能已按 fqdn 过滤，
	// 但有些服务商会返回整个 zone 的记录）
	records, err := p.GetRecords(ctx, fqdn, d.Type)
	if err != nil {
		log.Error("failed to query records",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"ipv6", ipv6Str, "err", err)
		return fmt.Errorf("failed to query records: %w", err)
	}

	log.Debug("DNS records query completed",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"record_count", len(records))

	found := false // 是否找到匹配的子域名 AAAA 记录

	for _, r := range records {
		// 过滤：只处理匹配目标子域名 + 目标类型的记录。
		// recordNameMatches 处理不同服务商返回的记录名格式差异。
		if !recordNameMatches(r.Name, fqdn, d.SubDomain) || r.Type != d.Type {
			continue
		}
		found = true

		log.Debug("comparing DNS record values",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"existing_value", r.Value, "new_value", ipv6Str,
			"record_id", r.ID, "record_type", r.Type)

		// IP 相同则更新缓存后跳过（多个 AAAA 记录时继续处理下一条）
		if ipv6Equal(r.Value, ipv6Str) {
			d.SetAddr(addr)
			log.Debug("IPv6 record already matches, no update needed",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"record_id", r.ID)
			continue
		}

		// IP 不同 → 修改记录
		err = p.ModifyRecord(ctx, fqdn, r.ID, d.Type, ipv6Str, d.TTL)
		if err != nil {
			log.Error("failed to modify record",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"ipv6", ipv6Str, "record_id", r.ID, "err", err)
			return fmt.Errorf("failed to modify record: %w", err)
		}
		d.SetAddr(addr)
		log.Info("IPv6 address changed, record modified",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"ipv6", ipv6Str, "record_id", r.ID)
	}

	// 目标子域名下无 AAAA 记录 → 新增
	if !found {
		log.Debug("no AAAA record found, adding new record",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"fqdn", fqdn, "ipv6", ipv6Str)

		err = p.AddRecord(ctx, fqdn, d.Type, ipv6Str, d.TTL)
		if err != nil {
			log.Error("failed to add record",
				"domain", d.Domain, "subdomain", d.SubDomain,
				"ipv6", ipv6Str, "err", err)
			return fmt.Errorf("failed to add record: %w", err)
		}
		d.SetAddr(addr)
		log.Info("IPv6 address changed, record added",
			"domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str)
	}

	return nil
}
