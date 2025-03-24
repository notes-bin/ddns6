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

// IPv6Getter 定义获取 IPv6 地址的接口，使用 context 支持取消操作
type IPv6Getter interface {
	GetIPv6Addr(ctx context.Context) (net.IP, error)
}

// Tasker 定义执行 DNS 更新任务的接口
type Tasker interface {
	Task(domain, subdomain, ipv6addr string) error
}

// Domain 表示一个域名及其相关配置
type Domain struct {
	Domain    string     `env:"DOMAIN" required:"true"` // 主域名
	SubDomain string     `env:"SUB_DOMAIN" default:"@"` // 子域名
	Type      string     `env:"TYPE" default:"AAAA"`    // 记录类型
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
func (d *Domain) UpdateRecord(ctx context.Context, ipv6Getter IPv6Getter, tasker Tasker) error {
	select {
	case <-ctx.Done():
		slog.Info("更新任务被取消", "domain", d.Domain, "subdomain", d.SubDomain)
		return ctx.Err()
	default:
		d.mu.Lock()
		defer d.mu.Unlock()

		addr, err := d.getIPv6Address(ctx, ipv6Getter)
		if err != nil {
			return d.handleError("获取 IPv6 地址失败", err)
		}

		if d.hasAddressChanged(addr) {
			return d.updateDNSRecord(tasker, addr)
		}
		slog.Info("IPv6 地址未改变，无需更新", "domain", d.Domain, "subdomain", d.SubDomain)
		return nil
	}
}

// getIPv6Address 获取 IPv6 地址，增加重试机制和日志记录
func (d *Domain) getIPv6Address(ctx context.Context, ipv6Getter IPv6Getter) (net.IP, error) {
	const maxRetries = 3 // 最大重试次数
	var addr net.IP
	var err error

	for i := range maxRetries {
		if ctx.Err() != nil {
			return nil, ctx.Err() // 如果上下文已取消，直接返回错误
		}

		slog.Info("尝试获取 IPv6 地址", "domain", d.Domain, "subdomain", d.SubDomain, "attempt", i+1)
		addr, err = ipv6Getter.GetIPv6Addr(ctx)

		if err == nil {
			if addr == nil {
				err = ErrEmptyIPv6Address
			} else {
				slog.Info("成功获取 IPv6 地址", "domain", d.Domain, "subdomain", d.SubDomain, "ipv6", addr.String())
				return addr, nil
			}
		}

		slog.Error("获取 IPv6 地址失败", "domain", d.Domain, "subdomain", d.SubDomain, "err", err, "attempt", i+1)
	}

	return nil, fmt.Errorf("多次尝试后仍无法获取 IPv6 地址: %w", err)
}

// updateDNSRecord 更新 DNS 记录
func (d *Domain) updateDNSRecord(tasker Tasker, addr net.IP) error {
	d.Addr = addr
	err := tasker.Task(d.Domain, d.SubDomain, d.Addr.String())
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

// handleError 处理错误并记录日志
func (d *Domain) handleError(msg string, err error) error {
	d.Err = err
	slog.Error(msg, "domain", d.Domain, "subdomain", d.SubDomain, "err", err)
	return err
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
