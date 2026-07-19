package providers

import (
	"context"
	"net"
)

// effectiveTTL 返回有效的 TTL 值，如果未设置则返回默认值 600
func (d *Domain) effectiveTTL() int {
	if d.TTL > 0 {
		return d.TTL
	}
	return 600
}

// hasAddressChanged 检查 IPv6 地址是否改变
func (d *Domain) hasAddressChanged(newAddr net.IP) bool {
	changed := d.Addr == nil || !d.Addr.Equal(newAddr)
	if d.Addr == nil {
		log.Debug("no cached address, update needed")
	} else if changed {
		log.Debug("IPv6 address has changed",
			"old_addr", d.Addr.String(), "new_addr", newAddr.String())
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

// UpdateRecord 更新 DNS 记录（DDNS 主入口）
// 先检查地址是否变化，若无变化则跳过更新
func (d *Domain) UpdateRecord(ctx context.Context, ipv6 net.IP, p DNSProvider) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.checkCancelled(ctx, "update"); err != nil {
		return err
	}

	if !d.hasAddressChanged(ipv6) {
		log.Info("IPv6 address unchanged, skipping update",
			"domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}

	return d.updateDNSRecord(ctx, p, ipv6)
}

// updateDNSRecord 执行实际 DNS 记录更新逻辑
// 先查询现有记录，同 IP 则跳过，不同 IP 则修改，无记录则新增
func (d *Domain) updateDNSRecord(ctx context.Context, p DNSProvider, addr net.IP) error {
	fqdn := d.fullDomain()
	ipv6Str := addr.String()

	log.Debug("querying existing DNS records",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "type", d.Type)

	records, err := p.GetRecords(ctx, fqdn, d.Type)
	if err != nil {
		return d.handleError("failed to query records", err, addr)
	}

	log.Debug("DNS records query completed",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"record_count", len(records))

	// 遍历记录，先按类型筛选，再比较值
	for _, r := range records {
		// 先按记录类型筛选，跳过不匹配类型的记录
		if r.Type != d.Type {
			continue
		}

		log.Debug("comparing DNS record values",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"existing_value", r.Value, "new_value", ipv6Str,
			"record_id", r.ID, "record_type", r.Type)

		// 归一化比较 IPv6 地址（服务商可能返回非规范形式）
		if ipv6Equal(r.Value, ipv6Str) {
			d.setAddr(addr)
			log.Info("IPv6 record already exists, no update needed",
				"domain", d.Domain, "subdomain", d.SubDomain)
			return nil
		}

		// 同类型记录但 IP 不同，修改
		err = p.ModifyRecord(ctx, fqdn, r.ID, d.Type, ipv6Str, r.TTL)
		if err != nil {
			return d.handleError("failed to modify record", err, addr)
		}
		d.setAddr(addr)
		log.Info("IPv6 address changed, DDNS modify completed",
			"domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str,
		)
		return nil
	}

	// 无 AAAA 记录，新增
	err = p.AddRecord(ctx, fqdn, d.Type, ipv6Str, d.effectiveTTL())
	if err != nil {
		return d.handleError("failed to add record", err, addr)
	}
	d.setAddr(addr)
	log.Info("IPv6 address changed, DDNS add completed",
		"domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str,
	)
	return nil
}
