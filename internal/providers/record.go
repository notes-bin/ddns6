package providers

import (
	"context"
	"fmt"
	"net"
	"sync"
)

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

// 默认值常量
const (
	DefaultTTL = 600 // DNS 记录默认 TTL（秒）
)

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

// Lock 锁定 Domain，供外部包保护并发访问
func (d *Domain) Lock() { d.mu.Lock() }

// Unlock 解锁 Domain
func (d *Domain) Unlock() { d.mu.Unlock() }

// FullDomain 返回完整的子域名（含主域名）
func (d *Domain) FullDomain() string { return d.fullDomain() }

// SetAddr 更新缓存的 IPv6 地址（拷贝防止别名）
func (d *Domain) SetAddr(addr net.IP) {
	d.Addr = make(net.IP, len(addr))
	copy(d.Addr, addr)
}
