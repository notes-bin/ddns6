// Package ddns 提供动态域名解析（DDNS）服务编排。
//
// 核心类型：RecordInfo（通用 DNS 记录）、DNSProvider（服务商接口）、
// Domain（域名配置）；入口为 RunService。
//
// 工作流程：
//
//	Linux: Netlink 监听地址变化 -> debounce 10s -> 获取 IPv6 -> 同步 DNS 记录
//	其他:  定时轮询 -> 获取 IPv6 -> 同步 DNS 记录
//
// 同步策略：查询目标子域名 AAAA 记录；IP 相同则跳过，不同则修改；无记录则新增。
//
// 使用示例（作为库调用）：
//
//	domains := []*ddns.Domain{
//	    {Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600},
//	}
//	p := tencent.NewDNSPod("your-secret-id", "your-secret-key")
//	err := ddns.RunService(domains, p, 5*time.Minute, ddns.DefaultIPv6Fetchers(), "")
//
// 新增运营商需实现 DNSProvider（4 个方法），并在 cmd/providers.go 注册。
package ddns

import (
	"context"
	"fmt"
	"net"
	"sync"
)

// RecordInfo 是 DNSProvider CRUD 方法的统一记录载体。
//
// 在服务编排层（record.go / processor.go）与运营商实现之间传递；
// 各运营商内部 API 结构体在接口边界处与 RecordInfo 相互转换。
//
// Zone 为根域名（来自 --domain）；非空时 provider 应优先使用 Zone，
// 而不是从 Name 推导根域名。
type RecordInfo struct {
	ID    string // 记录 ID（部分运营商创建后才有）
	Name  string // 完整记录名或主机名（因运营商而异）
	Zone  string // 根域名（如 example.com）；可选，空则从 Name 推导
	Type  string // 记录类型，如 AAAA
	Value string // 记录值，如 IPv6 地址
	TTL   int    // TTL（秒）
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

// Domain 表示待同步的域名配置及缓存的 IPv6 地址。
//
// 并发访问由内嵌的 mu 保护；通过 CheckAndSetAddr / AddrString 等导出方法读写缓存。
type Domain struct {
	Domain    string // 根域名
	SubDomain string // 子域名；"@" 表示根域名本身
	Type      string // 记录类型，通常为 AAAA
	TTL       int    // TTL（秒）
	Addr      net.IP // 最近一次成功同步的地址缓存
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
