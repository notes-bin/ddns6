// Package ddns 提供动态域名解析（DDNS）服务编排
//
// 本包定义了 DDNS 核心类型：RecordInfo（通用 DNS 记录）、DNSProvider（服务商接口）、
// Domain（域名配置），以及服务编排的入口 RunService。
//
// 使用示例（作为库调用）：
//
//	// 1. 创建域名配置
//	domains := []*ddns.Domain{
//	    {Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600},
//	    {Domain: "example.com", SubDomain: "@", Type: "AAAA", TTL: 600},
//	}
//
//	// 2. 创建 Provider（以 tencent 为例）
//	p := tencent.NewDNSPod("your-secret-id", "your-secret-key")
//
//	// 3. 启动服务
//	err := ddns.RunService(domains, p, 5*time.Minute, ddns.DefaultIPv6Fetchers, "")
//
// 新增 DNS 运营商需实现 DNSProvider 接口（4 个方法），然后在
// cmd/providers.go 的 providerFactories 列表中注册。
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

// DNSProvider DNS 服务商接口，提供 DNS 记录的增删改查操作。
//
// 新增 DNS 运营商需实现此接口的全部 4 个方法，然后
// 在 cmd/providers.go 的 providerFactories 列表中注册。
type DNSProvider interface {
	// GetRecords 查询 DNS 记录列表，按 domain 和 recordType 过滤
	GetRecords(ctx context.Context, domain, recordType string) ([]RecordInfo, error)
	// AddRecord 添加一条 DNS 记录
	AddRecord(ctx context.Context, record RecordInfo) error
	// ModifyRecord 修改一条 DNS 记录
	ModifyRecord(ctx context.Context, record RecordInfo) error
	// DeleteRecord 删除一条 DNS 记录
	DeleteRecord(ctx context.Context, record RecordInfo) error
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

// String 返回 Domain 的字符串表示（线程安全）。
func (d *Domain) String() string {
	d.mu.Lock()
	addr := d.Addr.String()
	d.mu.Unlock()
	return fmt.Sprintf("fullDomain: %s, type: %s, addr: %s", d.FullDomain(), d.Type, addr)
}

// FullDomain 返回完整的子域名（含主域名）。
func (d *Domain) FullDomain() string {
	if d.SubDomain == "" || d.SubDomain == "@" {
		return d.Domain
	}
	return fmt.Sprintf("%s.%s", d.SubDomain, d.Domain)
}

// CheckAndSetAddr 检查并更新缓存的 IPv6 地址，返回地址是否发生变化（线程安全）。
//
// 若新地址与缓存地址相同则返回 false，否则更新缓存并返回 true。
// 用于 SyncRecord 中判断是否需要触发 DNS 记录更新。
func (d *Domain) CheckAndSetAddr(newAddr net.IP) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Addr != nil && d.Addr.Equal(newAddr) {
		return false
	}
	d.Addr = make(net.IP, len(newAddr))
	copy(d.Addr, newAddr)
	return true
}

// AddrString 返回缓存 IP 地址的字符串表示（线程安全）。
func (d *Domain) AddrString() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.Addr.String()
}

// lock 内部加锁（非导出），供 SyncRecord 等包内函数在需要跨方法持锁时使用。
func (d *Domain) lock() { d.mu.Lock() }

// unlock 内部解锁（非导出）。
func (d *Domain) unlock() { d.mu.Unlock() }
