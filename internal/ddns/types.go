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
//	p := tencent.NewClient("your-secret-id", "your-secret-key")
//	err := ddns.RunService(domains, p, 5*time.Minute, ddns.DefaultIPv6Fetchers(), "", "")
//
// 新增运营商需实现 DNSProvider（4 个方法），并在 cmd/providers.go 注册。
package ddns

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
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

// ValueFromID 从复合 ID（格式 "任意前缀|值"）解析旧记录值；无分隔符则回退到 Value。
//
// deSEC / Hetzner 等运营商在 Modify 时用 ID 携带旧值以构造替换请求。
func (r RecordInfo) ValueFromID() string {
	if _, value, ok := strings.Cut(r.ID, "|"); ok {
		return value
	}
	return r.Value
}

// DNSProvider 定义 DNS 服务商的记录增删改查接口。
//
// 新增运营商需实现全部 4 个方法，并在 cmd/providers.go 的 providerFactories 中注册。
//
// 并发安全：同一 Client 可能被 syncAllDomains / clean 等路径并发调用，
// 实现须对共享可变状态（缓存、RMW 写路径、token）自行同步。
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
// Domain/SubDomain/Type/TTL 在服务启动后视为只读，可无锁并发读。
// addr 缓存由 mu 保护；外部应通过 CheckAndSetAddr / AddrString 访问。
type Domain struct {
	Domain    string // 根域名
	SubDomain string // 子域名；"@" 表示根域名本身
	Type      string // 记录类型，通常为 AAAA
	TTL       int    // TTL（秒）
	addr      net.IP // 最近一次成功同步的地址缓存（须经方法或包内锁访问）
	mu        sync.Mutex
}

// DefaultTTL 为 DNS 记录默认 TTL（秒）。
const DefaultTTL = 600

// RecordTTL 返回有效 TTL；传入 0 或负值时回退为 DefaultTTL。
func RecordTTL(ttl int) int {
	if ttl > 0 {
		return ttl
	}
	return DefaultTTL
}

// String 返回 Domain 的可读描述（线程安全）。
func (d *Domain) String() string {
	d.mu.Lock()
	addr := d.addr.String()
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
	if d.addr != nil && d.addr.Equal(newAddr) {
		return false
	}
	d.addr = slices.Clone(newAddr)
	return true
}

// AddrString 返回缓存 IP 的字符串形式（线程安全）。
func (d *Domain) AddrString() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.addr.String()
}

// lock 供 SyncRecord 等包内函数在跨方法持锁时使用。
func (d *Domain) lock() { d.mu.Lock() }

// unlock 与 lock 配对解锁。
func (d *Domain) unlock() { d.mu.Unlock() }
