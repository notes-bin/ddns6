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

// fullDomain 返回完整的子域名（含主域名）
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
	d.Addr = make(net.IP, len(addr))
	copy(d.Addr, addr)
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
