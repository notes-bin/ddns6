package providers

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

var log = slog.With("module", "providers")

// RecordInfo 通用 DNS 记录类型
type RecordInfo struct {
	ID    string
	Name  string
	Type  string
	Value string
	TTL   int
}

// RecordAdder 添加 DNS 记录
type RecordAdder interface {
	AddRecord(ctx context.Context, domain, recordType, value string, ttl int) error
}

// RecordModifier 修改 DNS 记录
type RecordModifier interface {
	ModifyRecord(ctx context.Context, domain, recordID, recordType, value string, ttl int) error
}

// RecordDeleter 删除 DNS 记录
type RecordDeleter interface {
	DeleteRecord(ctx context.Context, domain, recordID string) error
}

// RecordQuerier 查询 DNS 记录
type RecordQuerier interface {
	GetRecords(ctx context.Context, domain, recordType string) ([]RecordInfo, error)
}

// DNSProvider DNS 服务商完整接口
type DNSProvider interface {
	RecordAdder
	RecordModifier
	RecordDeleter
	RecordQuerier
}

// Domain 表示一个域名及其相关配置
type Domain struct {
	Domain    string     `env:"DOMAIN" required:"true"` // 主域名
	SubDomain string     `env:"SUB_DOMAIN" default:"@"` // 子域名
	Type      string     `env:"TYPE" default:"AAAA"`    // 记录类型
	TTL       int        `env:"TTL" default:"600"`      // TTL，默认 600 秒
	Addr      net.IP     // IPv6 地址
	mu        sync.Mutex // 互斥锁
}

// String 返回 Domain 的字符串表示
func (d *Domain) String() string {
	fullDomain := d.Domain
	if d.SubDomain != "" && d.SubDomain != "@" {
		fullDomain = fmt.Sprintf("%s.%s", d.SubDomain, d.Domain)
	}
	return fmt.Sprintf("fullDomain: %s, type: %s, addr: %s", fullDomain, d.Type, d.Addr)
}

func (d *Domain) fullDomain() string {
	if d.SubDomain == "" || d.SubDomain == "@" {
		return d.Domain
	}
	return fmt.Sprintf("%s.%s", d.SubDomain, d.Domain)
}

// checkCancelled 检查 context 是否已取消，取消时记录日志并返回错误
func (d *Domain) checkCancelled(ctx context.Context, action string) error {
	select {
	case <-ctx.Done():
		log.Info(action+" task cancelled", "domain", d.Domain, "subdomain", d.SubDomain)
		return ctx.Err()
	default:
		return nil
	}
}

// setAddr 更新缓存的 IPv6 地址
func (d *Domain) setAddr(addr net.IP) {
	d.setAddr(addr)
}

func (d *Domain) effectiveTTL() int {
	if d.TTL > 0 {
		return d.TTL
	}
	return 600
}

// AddDomainRecord 添加 DNS 解析记录
func (d *Domain) AddDomainRecord(ctx context.Context, ipv6 net.IP, p DNSProvider, ttl int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.checkCancelled(ctx, "add"); err != nil {
		return err
	}

	fqdn := d.fullDomain()
	ipv6Str := ipv6.String()

	log.Debug("adding DNS record",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "type", d.Type, "ipv6", ipv6Str, "ttl", ttl)

	if err := p.AddRecord(ctx, fqdn, d.Type, ipv6Str, ttl); err != nil {
		return d.handleError("failed to add record", err, ipv6)
	}

	d.setAddr(ipv6)
	log.Info("DNS record added successfully",
		"domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str)
	return nil
}

// ModifyDomainRecord 修改 DNS 解析记录
func (d *Domain) ModifyDomainRecord(ctx context.Context, ipv6 net.IP, p DNSProvider, recordID string, ttl int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.checkCancelled(ctx, "modify"); err != nil {
		return err
	}

	fqdn := d.fullDomain()
	ipv6Str := ipv6.String()

	log.Debug("modifying DNS record",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "record_id", recordID, "type", d.Type, "ipv6", ipv6Str)

	if err := p.ModifyRecord(ctx, fqdn, recordID, d.Type, ipv6Str, ttl); err != nil {
		return d.handleError("failed to modify record", err, ipv6)
	}

	d.setAddr(ipv6)
	log.Info("DNS record modified successfully",
		"domain", d.Domain, "subdomain", d.SubDomain, "record_id", recordID, "ipv6", ipv6Str)
	return nil
}

// DeleteDomainRecord 删除 DNS 解析记录
func (d *Domain) DeleteDomainRecord(ctx context.Context, p DNSProvider, recordID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.checkCancelled(ctx, "delete"); err != nil {
		return err
	}

	fqdn := d.fullDomain()

	log.Debug("deleting DNS record",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "record_id", recordID)

	if err := p.DeleteRecord(ctx, fqdn, recordID); err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	log.Info("DNS record deleted successfully",
		"domain", d.Domain, "subdomain", d.SubDomain, "record_id", recordID)
	return nil
}

// GetDomainRecords 查询 DNS 解析记录
func (d *Domain) GetDomainRecords(ctx context.Context, p DNSProvider) ([]RecordInfo, error) {
	fqdn := d.fullDomain()

	log.Debug("querying DNS records",
		"domain", d.Domain, "subdomain", d.SubDomain, "fqdn", fqdn, "type", d.Type)

	records, err := p.GetRecords(ctx, fqdn, d.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to query records: %w", err)
	}

	log.Debug("DNS records query completed",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"record_count", len(records))

	return records, nil
}

// UpdateRecord 更新 DNS 记录
func (d *Domain) UpdateRecord(ctx context.Context, ipv6 net.IP, p DNSProvider) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.checkCancelled(ctx, "update"); err != nil {
		return err
	}

	if !d.hasAddressChanged(ipv6) {
		log.Info("IPv6 address unchanged, skipping update", "domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}

	return d.updateDNSRecord(ctx, p, ipv6)
}

// updateDNSRecord 更新 DNS 记录
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
			log.Info("IPv6 record already exists, no update needed", "domain", d.Domain, "subdomain", d.SubDomain)
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

// handleError 处理错误并记录日志
func (d *Domain) handleError(action string, err error, addr net.IP) error {
	log.Error("DDNS "+action,
		"domain", d.Domain,
		"subdomain", d.SubDomain,
		"ipv6", addr.String(),
		"err", err,
	)
	return fmt.Errorf("%s: %w", action, err)
}
