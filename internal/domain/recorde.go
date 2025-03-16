package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// IPv6Getter 定义获取 IPv6 地址的接口
type IPv6Getter interface {
	GetIPv6Addr() (net.IP, error)
}

// Tasker 定义执行 DNS 更新任务的接口
type Tasker interface {
	Task(domain, subdomain, ipv6addr string) error
}

// UpdateRecorder 定义更新 DNS 记录的接口
type UpdateRecorder interface {
	UpdateRecord(ctx context.Context, ipv6Getter IPv6Getter, tasker Tasker, err error)
}

// Domain 表示一个域名及其相关配置
type Domain struct {
	Domain    string     `env:"DOMAIN"`    // 主域名
	SubDomain string     `env:"SUBDOMAIN"` // 子域名
	Type      string     `env:"TYPE"`      // 记录类型
	Addr      net.IP     // IPv6 地址
	Err       error      // 错误信息
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
func (d *Domain) UpdateRecord(ctx context.Context, ipv6Getter IPv6Getter, tasker Tasker, err error) {
	select {
	case <-ctx.Done():
		return
	default:
		d.mu.Lock()
		defer d.mu.Unlock()

		// 获取 IPv6 地址
		addr, err := ipv6Getter.GetIPv6Addr()
		if err != nil {
			d.handleError("获取 IPv6 地址失败", err)
			return
		}

		// 检查是否获取到地址
		if addr == nil {
			d.handleError("获取到的 IPv6 地址为空", fmt.Errorf("获取到的 IPv6 地址为空"))
			return
		}

		// 检查 IPv6 地址是否改变
		if d.hasAddressChanged(addr) {
			d.Addr = addr
			if err := tasker.Task(d.Domain, d.SubDomain, d.Addr.String()); err != nil {
				d.handleTaskError(err, err)
			} else {
				slog.Info("IPv6 地址发生变化, ddns配置完成",
					"domain", d.Domain,
					"subdomain", d.SubDomain,
					"ipv6", d.Addr.String(),
				)
			}
		}
	}
}

// hasAddressChanged 检查 IPv6 地址是否改变
func (d *Domain) hasAddressChanged(newAddr net.IP) bool {
	return d.Addr == nil || !d.Addr.Equal(newAddr)
}

// handleError 处理错误并记录日志
func (d *Domain) handleError(msg string, err error) {
	d.Err = err
	slog.Error(msg, "err", err)
}

// handleTaskError 处理任务错误并记录日志
func (d *Domain) handleTaskError(taskErr, expectedErr error) {
	if errors.Is(taskErr, expectedErr) {
		slog.Info("IPv6 地址未改变, 无法配置ddns",
			"domain", d.Domain,
			"subdomain", d.SubDomain,
			"ipv6", d.Addr.String(),
		)
	} else {
		slog.Error("配置ddns解析失败",
			"domain", d.Domain,
			"subdomain", d.SubDomain,
			"ipv6", d.Addr.String(),
			"err", taskErr,
		)
	}
}
