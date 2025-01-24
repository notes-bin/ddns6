package main

import (
	"flag"
	"net"
	"time"

	"github.com/notes-bin/ddns6/utils"
)

// IPv6Geter 接口定义了一个方法 GetIPV6Addr，用于获取 IPv6 地址列表。
// 实现该接口的类型需要提供一个返回 IPv6 地址切片和错误信息的方法。
type IPv6Geter interface {
	// GetIPV6Addr 返回一个包含 IPv6 地址的切片以及可能发生的错误。
	// ipv6: []*net.IP 类型的切片，包含获取到的 IPv6 地址。
	// err: 如果在获取地址过程中发生错误，将返回相应的错误信息。
	GetIPV6Addr() (ipv6 []*net.IP, err error)
}

// DNS 结构体表示一个 DNS 记录，包含以下字段：
// - Domain: 域名
// - Name: 记录名称
// - Type: 记录类型，例如 A、AAAA、CNAME 等
// - Addr: 指向 IP 地址的指针
type dns struct {
	Domain    string
	SubDomain string
	Type      string
	Addr      []*net.IP
}

func NewDns(domain, subdomain string) *dns {
	if subdomain == "" {
		subdomain = "@"
	}
	return &dns{
		Domain:    domain,
		SubDomain: subdomain,
		Type:      "AAAA",
		Addr:      make([]*net.IP, 0),
	}
}

func (d *dns) update(ip IPv6Geter, duration time.Duration) {
	ticker := time.NewTicker(duration)
	for range ticker.C {
		ipv6, err := ip.GetIPV6Addr()
		if err != nil {
			panic(err)
		}
		d.Addr = ipv6
	}
}

func main() {
	ipv6 := flag.String("ipv6", "dns", "service name, dns, iface, site")
	pdns := flag.String("dns", "", "dns name")
	site := flag.String("site", "", "site name")
	iface := flag.String("iface", "", "interface name")
	domain := flag.String("domain", "", "domain name")
	subdomain := flag.String("subdomain", "", "subdomain name")
	interval := flag.Int("interval", 10, "interval time")
	flag.Parse()

	dns := NewDns(*domain, *subdomain)
	duration := time.Duration(*interval) * time.Minute

	switch *ipv6 {
	case "dns":
		ip := utils.NewPublicDNS(*pdns)
		dns.update(ip, duration)
	case "iface":
		ip := utils.NewIface(*iface)
		dns.update(ip, duration)
	case "site":
		ip := utils.NewSite(*site)
		dns.update(ip, duration)
	default:
		panic("service not found")
	}
}
