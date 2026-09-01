// Package ddns 提供动态 DNS（DDNS）服务编排：IPv6 获取、地址变化触发与 DNS 记录同步。
//
// 核心类型：RecordInfo（统一记录载体）、DNSProvider（运营商接口）、Domain（待同步域名）；
// 服务入口为 RunService。运营商实现位于 internal/providers，本包不依赖具体厂商。
//
// 工作流程：
//
//	Linux: Netlink 监听地址变化 -> debounce 10s -> 获取 IPv6 -> 同步 DNS 记录
//	其他:  定时轮询 -> 获取 IPv6 -> 同步 DNS 记录
//
// 同步策略：查询目标子域名记录；IP 相同则跳过，不同则修改；无记录则新增。
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

// Key 返回用于去重的唯一键（ID+Name+Type+Value）。
func (r RecordInfo) Key() string {
	return r.ID + "|" + r.Name + "|" + r.Type + "|" + r.Value
}

// DNSProvider 定义 DNS 服务商的记录增删改查接口。
//
// 新增运营商需实现全部 4 个方法，并在 cmd/providers.go 的 providerFactories 中注册。
type DNSProvider interface {
	// GetRecords 按 domain 与 recordType 查询记录列表。
	GetRecords(ctx context.Context, domain, recordType string) ([]RecordInfo, error)
	// AddRecord 添加一条 DNS 记录。
	AddRecord(ctx context.Context, record RecordInfo) error
	// ModifyRecord 修改一条 DNS 记录。
	ModifyRecord(ctx context.Context, record RecordInfo) error
	// DeleteRecord 删除一条 DNS 记录。
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

// DefaultTTL 为 DNS 记录默认 TTL（秒）。
const DefaultTTL = 600

// String 返回 Domain 的可读描述（线程安全）。
func (d *Domain) String() string {
	d.mu.Lock()
	addr := d.Addr.String()
	d.mu.Unlock()
	return fmt.Sprintf("fullDomain: %s, type: %s, addr: %s", d.FullDomain(), d.Type, addr)
}

// FullDomain 返回完整域名（子域名 + 根域名；"@" 或空子域名时仅为根域名）。
func (d *Domain) FullDomain() string {
	if d.SubDomain == "" || d.SubDomain == "@" {
		return d.Domain
	}
	return fmt.Sprintf("%s.%s", d.SubDomain, d.Domain)
}

// CheckAndSetAddr 在地址变化时更新缓存，并返回是否发生变化（线程安全）。
//
// 新地址与缓存相同则返回 false；否则更新缓存并返回 true。
// SyncRecord 据此决定是否发起 DNS 更新。
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

// AddrString 返回缓存 IP 的字符串形式（线程安全）。
func (d *Domain) AddrString() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.Addr.String()
}

// lock 供 SyncRecord 等包内函数在跨方法持锁时使用。
func (d *Domain) lock() { d.mu.Lock() }

// unlock 与 lock 配对解锁。
func (d *Domain) unlock() { d.mu.Unlock() }
