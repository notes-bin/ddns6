package main

import (
	"flag"
	"net"
	"time"

	"github.com/notes-bin/ddns6/pkg/tencent"
	"github.com/notes-bin/ddns6/utils"
)

// IPv6Geter 接口定义了一个方法 GetIPV6Addr，用于获取 IPv6 地址列表。
// 实现该接口的类型需要提供一个返回 IPv6 地址切片和错误信息的方法。
type IPv6Geter interface {
	GetIPV6Addr() (ipv6 []*net.IP, err error)
}

type Jober interface {
	Job(domain string)
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

func (d *dns) monitor(ip IPv6Geter, job Jober, duration time.Duration) {
	ticker := time.NewTicker(duration)
	for range ticker.C {
		ipv6, err := ip.GetIPV6Addr()
		if err != nil {
			panic(err)
		}
		d.Addr = ipv6

		job.Job(d.Domain)
	}
}

func main() {
	ipv6 := flag.String("ipv6", "dns", "dns, iface, site")
	pdns := flag.String("dns", "", "dns name")
	site := flag.String("site", "", "site name")
	iface := flag.String("iface", "", "interface name")
	domain := flag.String("domain", "", "domain name")
	subdomain := flag.String("subdomain", "", "subdomain name")
	interval := flag.Int("interval", 10, "interval time")
	service := flag.String("service", "tencent", "service name")
	accessKey := flag.String("ak", "", "access key")
	secretKey := flag.String("sk", "", "secret key")
	flag.Parse()

	dns := NewDns(*domain, *subdomain)
	duration := time.Duration(*interval) * time.Minute

	var ip IPv6Geter
	switch *ipv6 {
	case "dns":
		ip = utils.NewPublicDNS(*pdns)
	case "iface":
		ip = utils.NewIface(*iface)
	case "site":
		ip = utils.NewSite(*site)
	default:
		panic("service not found")
	}

	var job Jober
	switch *service {
	case "tencent":
		job = tencent.New(*accessKey, *secretKey)
	default:
		panic("service not found")
	}

	dns.monitor(ip, job, duration)
}
