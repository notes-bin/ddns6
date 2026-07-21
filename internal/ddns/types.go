// Package ddns 提供动态域名解析（DDNS）服务编排
//
// 本包定义了 DDNS 核心类型：RecordInfo（通用 DNS 记录）、DNSProvider（服务商接口）、
// Domain（域名配置），以及服务编排的入口 RunService。
package ddns

import (
	"context"
	"fmt"
	"net"
	"sync"
)

// RecordInfo 通用 DNS 记录类型
//
// 作为 DNSProvider 接口中所有 CRUD 方法的统一数据载体，在服务编排层（sync.go）和
// 运营商实现层（providers/ 下各子包）之间传递数据。各运营商内部有各自的 API 结构体，
// 在接口方法边界处与 RecordInfo 相互转换。
//
// Zone 字段存储根域名（来自 --domain 参数），供 provider 的 SplitDomain 操作使用。
// 当 Zone 非空时，provider 应优先使用 Zone 而不是从 Name 中推导根域名。
type RecordInfo struct {
	ID    string
	Name  string
	Zone  string // 根域名（如 example.com），可选，为空时回退到从 Name 推导
	Type  string
	Value string
	TTL   int
}

// Key 返回用于去重的唯一键。
func (r RecordInfo) Key() string {
	return r.ID + "|" + r.Name + "|" + r.Type + "|" + r.Value
}

// DNSRecordGetter 提供 DNS 记录查询能力
type DNSRecordGetter interface {
	// GetRecords 查询 DNS 记录列表，按 domain 和 recordType 过滤
	GetRecords(ctx context.Context, domain, recordType string) ([]RecordInfo, error)
}

// DNSRecordAdder 提供 DNS 记录新增能力
type DNSRecordAdder interface {
	// AddRecord 添加一条 DNS 记录
	AddRecord(ctx context.Context, record RecordInfo) error
}

// DNSRecordModifier 提供 DNS 记录修改能力
type DNSRecordModifier interface {
	// ModifyRecord 修改一条 DNS 记录
	ModifyRecord(ctx context.Context, record RecordInfo) error
}

// DNSRecordDeleter 提供 DNS 记录删除能力
type DNSRecordDeleter interface {
	// DeleteRecord 删除一条 DNS 记录
	DeleteRecord(ctx context.Context, record RecordInfo) error
}

// DNSProvider DNS 服务商完整接口
//
// 组合所有子接口，提供 DNS 记录的增删改查操作。
// 消费者可根据需要依赖具体的子接口，而非完整的 DNSProvider。
type DNSProvider interface {
	DNSRecordGetter
	DNSRecordAdder
	DNSRecordModifier
	DNSRecordDeleter
}

// Domain 表示一个域名及其相关配置
//
// 包含域名、子域名、记录类型、TTL 和缓存的 IP 地址。内嵌 sync.Mutex 保护并发访问。
type Domain struct {
	Domain    string
	SubDomain string
	Type      string
	TTL       int
	Addr      net.IP
	mu        sync.Mutex
}

// DefaultTTL DNS 记录默认 TTL（秒）
const DefaultTTL = 600

// String 返回 Domain 的字符串表示
func (d *Domain) String() string {
	return fmt.Sprintf("fullDomain: %s, type: %s, addr: %s", d.FullDomain(), d.Type, d.Addr)
}

// FullDomain 返回完整的子域名（含主域名）。
func (d *Domain) FullDomain() string {
	if d.SubDomain == "" || d.SubDomain == "@" {
		return d.Domain
	}
	return fmt.Sprintf("%s.%s", d.SubDomain, d.Domain)
}

// Lock 锁定 Domain，供外部包保护并发访问。
func (d *Domain) Lock() { d.mu.Lock() }

// Unlock 解锁 Domain。
func (d *Domain) Unlock() { d.mu.Unlock() }

// SetAddr 更新缓存的 IPv6 地址（拷贝防止别名）
func (d *Domain) SetAddr(addr net.IP) {
	d.Addr = make(net.IP, len(addr))
	copy(d.Addr, addr)
}
