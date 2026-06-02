package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// 定义自定义错误类型
var ErrEmptyIPv6Address = errors.New("获取到的 IPv6 地址为空")

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

// UpdateRecord 更新 DNS 记录
func (d *Domain) UpdateRecord(ctx context.Context, ipv6 net.IP, p DNSProvider) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-ctx.Done():
		slog.Info("更新任务被取消", "domain", d.Domain, "subdomain", d.SubDomain)
		return ctx.Err()
	default:
	}

	if !d.hasAddressChanged(ipv6) {
		slog.Info("IPv6 地址未改变，无需更新", "domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}

	return d.updateDNSRecord(ctx, p, ipv6)
}

// updateDNSRecord 更新 DNS 记录
func (d *Domain) updateDNSRecord(ctx context.Context, p DNSProvider, addr net.IP) error {
	fqdn := d.fullDomain()
	ipv6Str := addr.String()

	slog.Debug("查询现有 DNS 记录",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"fqdn", fqdn, "type", d.Type)

	records, err := p.GetRecords(ctx, fqdn, d.Type)
	if err != nil {
		return d.handleError("查询记录失败", err, addr)
	}

	slog.Debug("DNS 记录查询完成",
		"domain", d.Domain, "subdomain", d.SubDomain,
		"record_count", len(records))

	// 遍历记录，先检查是否需要更新
	for _, r := range records {
		slog.Debug("比对 DNS 记录值",
			"domain", d.Domain, "subdomain", d.SubDomain,
			"existing_value", r.Value, "new_value", ipv6Str,
			"record_id", r.ID, "record_type", r.Type)
		// 归一化比较 IPv6 地址（服务商可能返回非规范形式）
		if ipv6Equal(r.Value, ipv6Str) {
			d.Addr = make(net.IP, len(addr))
			copy(d.Addr, addr)
			slog.Info("IPv6 记录已存在，无需更新", "domain", d.Domain, "subdomain", d.SubDomain)
			return nil
		}

		// 同类型记录但 IP 不同，修改
		if r.Type == d.Type {
			err = p.ModifyRecord(ctx, fqdn, r.ID, d.Type, ipv6Str, r.TTL)
			if err != nil {
				return d.handleError("修改记录失败", err, addr)
			}
			d.Addr = make(net.IP, len(addr))
			copy(d.Addr, addr)
			slog.Info("IPv6 地址发生变化，DDNS 修改完成",
				"domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str,
			)
			return nil
		}
	}

	// 无 AAAA 记录，新增
	err = p.AddRecord(ctx, fqdn, d.Type, ipv6Str, 600)
	if err != nil {
		return d.handleError("添加记录失败", err, addr)
	}
	d.Addr = make(net.IP, len(addr))
	copy(d.Addr, addr)
	slog.Info("IPv6 地址发生变化，DDNS 添加完成",
		"domain", d.Domain, "subdomain", d.SubDomain, "ipv6", ipv6Str,
	)
	return nil
}

// hasAddressChanged 检查 IPv6 地址是否改变
func (d *Domain) hasAddressChanged(newAddr net.IP) bool {
	changed := d.Addr == nil || !d.Addr.Equal(newAddr)
	if d.Addr == nil {
		slog.Debug("无缓存地址，需要更新")
	} else if changed {
		slog.Debug("IPv6 地址已变化",
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
	slog.Error("DDNS "+action,
		"domain", d.Domain,
		"subdomain", d.SubDomain,
		"ipv6", addr.String(),
		"err", err,
	)
	return fmt.Errorf("%s: %w", action, err)
}
