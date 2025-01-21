// get ipv6 address
// use three methods, from network card, ipv6 website, dns server
package utils

import (
	"fmt"
	"io"
	"net"
	"net/http"
)

type iface struct {
	Name string
	ipv6 []*net.IP
}

func NewIface(name string) *iface {
	return &iface{
		Name: name,
	}
}

func (i *iface) GetIPV6Addr() error {
	iface, err := net.InterfaceByName(i.Name)
	if err != nil {
		fmt.Println(err)
		return err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		fmt.Println(err)
		return err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipNet.IP
		if ip.To4() == nil && ip.IsGlobalUnicast() {
			i.ipv6 = append(i.ipv6, &ip)
		}
	}
	return nil
}

type site struct {
	urls []string
	ipv6 []*net.IP
}

func NewSite(urls ...string) *site {
	u := []string{"https://6.ipw.cn"}
	if len(urls) > 0 {
		u = append(u, urls...)
	}

	return &site{
		urls: u,
	}
}

func (s *site) GetIPV6Addr() error {
	for _, url := range s.urls {
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println("request err -> ", err)
			return err
		}

		defer resp.Body.Close()
		res, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("read err -> ", err)
			return err
		}
		ipv6 := net.ParseIP(string(res))
		s.ipv6 = append(s.ipv6, &ipv6)
		if ipv6.To16() != nil {
			break
		}
	}
	return nil
}

type publicDns struct {
	port int
	dns  []string
	ipv6 []*net.IP
}

func NewPublicDNS(dns ...string) *publicDns {
	d := []string{"2400:3200:baba::1", "2001:4860:4860::8888"}
	if len(dns) > 0 {
		d = append(dns, d...)
	}
	return &publicDns{
		port: 53,
		dns:  d,
		ipv6: make([]*net.IP, 0),
	}

}

func (p *publicDns) GetIPV6Addr() error {
	// 连接到一个IPv6的DNS服务器，例如Google的公共DNS服务器
	for _, ip := range p.dns {
		dnsServer := fmt.Sprintf("[%s]:%d", ip, p.port)
		conn, err := net.Dial("udp6", dnsServer)
		if err != nil {
			fmt.Println("Error connecting to DNS server:", err)
			continue
		}
		defer conn.Close()

		// 获取本机的IPv6地址
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		p.ipv6 = append(p.ipv6, &localAddr.IP)
		if localAddr.IP.To16() != nil {
			break
		}
	}
	return nil
}
