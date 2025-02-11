package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

type IPv6Getter interface {
	GetIPV6Addr() ([]*net.IP, error)
}

type Tasker interface {
	Task(string, string, string) error
}

type Domain struct {
	Domain    string
	SubDomain string
	Type      string
	Addr      []*net.IP
	Err       error
	sync.Mutex
}

func (d *Domain) String() string {
	fullDomain := d.Domain
	if d.SubDomain != "" {
		fullDomain = fmt.Sprintf("%s.%s", d.SubDomain, d.Domain)
	}
	return fmt.Sprintf("fullDomain: %s, type: %s, addr: %s", fullDomain, d.Type, d.Addr)
}

func (d *Domain) UpdateRecord(ctx context.Context, ipv6Getter IPv6Getter, t Tasker, e error) {
	select {
	case <-ctx.Done():
		return
	default:
		d.Lock()
		defer d.Unlock()
		addr, err := ipv6Getter.GetIPV6Addr()
		if err != nil {
			slog.Error("获取 IPv6 地址失败", "err", err)
			d.Err = err
			return
		}

		// 确保获取到 addr
		if len(addr) == 0 {
			slog.Warn("获取到的 IPv6 地址为空")
			d.Err = fmt.Errorf("获取到的 IPv6 地址为空")
			return
		}

		// 检查 IPv6 地址是否改变, 如果发生改变, 则更新记录, 否则不更新
		if d.Addr == nil || !d.Addr[0].Equal(*addr[0]) {
			d.Addr = addr
			if err := t.Task(d.Domain, d.SubDomain, d.Addr[0].String()); err != nil {
				if errors.Is(err, e) {
					slog.Info("IPv6 地址未改变, 无法配置ddns", "domain", d.Domain, "subdomain", d.SubDomain, "ipv6", d.Addr[0].String())
				} else {
					slog.Error("配置ddns解析失败", "err", err)
				}
			} else {
				slog.Info("IPv6 地址发生变化, ddns配置完成", "domain", d.Domain, "subdomain", d.SubDomain, "ipv6", d.Addr[0].String())
			}
		}
		return
	}
}
