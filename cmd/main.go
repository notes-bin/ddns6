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
	domain := flag.String("domain", "", "设置域名")
	subdomain := flag.String("subdomain", "@", "设置子域名")
	ipv6 := flag.String("ipv6", "dns", "获取IPv6地址的方式, 可以是dns、site或iface,默认是dns")
	publicDns := flag.String("public-dns", "", "添加自定义公共IPv6 DNS, 默认包含阿里云和谷歌DNS")
	site := flag.String("site", "", "添加一个可以查询IPv6地址的自定义网站, 默认是https://6.ipw.cn")
	iface := flag.String("iface", "eth0", "设备的物理网卡名称")
	interval := flag.Int("interval", 10, "DDNS更新周期, 单位: 分钟")
	flag.Parse()

	var job Jober
	var ip IPv6Geter
	switch *ipv6 {
	case "dns":
		ip = utils.NewPublicDNS(*publicDns)
	case "site":
		ip = utils.NewSite(*site)
	case "iface":
		ip = utils.NewIface(*iface)
	default:
		panic("ipv6 must be dns or site or iface")
	}

	args := flag.Args()
	if len(args) == 0 {
		panic("请指定子命令")
	}

	switch args[0] {
	case "tencent":
		cmd := newSubCmd("tencent", "腾讯云")
		cmd.String("secret-id", "", "腾讯云 API 密钥 ID")
		cmd.String("secret-key", "", "腾讯云 API 密钥 Key")
		cmd.Parse(args[1:])
	default:
		panic("子命令必须为 tencent ...")
	}

	duration := time.Duration(*interval) * time.Minute
	dns := NewDns(*domain, *subdomain)

	dns.monitor(ip, job, duration)
}
