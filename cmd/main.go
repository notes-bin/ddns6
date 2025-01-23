package main

import (
	"net"
)

type IPv6Geter interface {
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

func NewDNS(domain, subdomain string, ip IPv6Geter) *dns {
	ipv6, err := ip.GetIPV6Addr()
	if err != nil {
		panic(err)
	}

	return &dns{
		Domain:    domain,
		SubDomain: subdomain,
		Type:      "AAAA",
		Addr:      ipv6,
	}
}

func main() {

}
