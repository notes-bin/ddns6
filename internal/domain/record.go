package domain

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

// Tasker 定义执行 DNS 更新任务的接口
type Tasker interface {
	Task(ctx context.Context, domain, subdomain, ipv6addr string) error
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
	if d.SubDomain != "" {
		fullDomain = fmt.Sprintf("%s.%s", d.SubDomain, d.Domain)
	}
	return fmt.Sprintf("fullDomain: %s, type: %s, addr: %s", fullDomain, d.Type, d.Addr)
}

// UpdateRecord 更新 DNS 记录
func (d *Domain) UpdateRecord(ctx context.Context, ipv6 net.IP, tasker Tasker) error {
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

	return d.updateDNSRecord(ctx, tasker, ipv6)
}

// updateDNSRecord 更新 DNS 记录
func (d *Domain) updateDNSRecord(ctx context.Context, tasker Tasker, addr net.IP) error {
	d.Addr = make(net.IP, len(addr))
	copy(d.Addr, addr)

	err := tasker.Task(ctx, d.Domain, d.SubDomain, d.Addr.String())
	if err != nil {
		return d.handleTaskError(err)
	}
	slog.Info("IPv6 地址发生变化，DDNS 配置完成",
		"domain", d.Domain,
		"subdomain", d.SubDomain,
		"ipv6", d.Addr.String(),
	)
	return nil
}

// hasAddressChanged 检查 IPv6 地址是否改变
func (d *Domain) hasAddressChanged(newAddr net.IP) bool {
	return d.Addr == nil || !d.Addr.Equal(newAddr)
}

// handleTaskError 处理任务错误并记录日志
func (d *Domain) handleTaskError(taskErr error) error {
	slog.Error("配置 DDNS 解析失败",
		"domain", d.Domain,
		"subdomain", d.SubDomain,
		"ipv6", d.Addr.String(),
		"err", taskErr,
	)
	return taskErr
}
