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
	name string
}

func NewIface(name string) *iface {
	return &iface{
		name: name,
	}
}

func (i *iface) GetIPV6Addr() (ipv6 []*net.IP, err error) {
	//TODO: 会获取多个ipv6地址,需要过滤
	iface, err := net.InterfaceByName(i.name)
	if err != nil {
		fmt.Println(err)
		return
	}

	addrs, err := iface.Addrs()
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipNet.IP
		if ip.To4() == nil && ip.IsGlobalUnicast() {
			ipv6 = append(ipv6, &ip)
		}
	}
	return
}

type site struct {
	urls []string
}

func NewSite(urls ...string) *site {
	u := []string{"https://6.ipw.cn"}
	if len(urls) > 0 {
		u = append(urls, u...)
	}

	return &site{
		urls: u,
	}
}

func (s *site) GetIPV6Addr() (ipv6 []*net.IP, err error) {
	for _, url := range s.urls {
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println("request err -> ", err)
			return nil, err
		}

		defer resp.Body.Close()
		res, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("read err -> ", err)
			return nil, err
		}
		ip := net.ParseIP(string(res))
		ipv6 = append(ipv6, &ip)
		if ip.To16() != nil {
			break
		}
	}
	return
}

type publicDns struct {
	port int
	dns  []string
}

func NewPublicDNS(dns ...string) *publicDns {
	d := []string{"2400:3200:baba::1", "2001:4860:4860::8888"}
	if len(dns) > 0 {
		d = append(dns, d...)
	}
	return &publicDns{
		port: 53,
		dns:  d,
	}

}

func (p *publicDns) GetIPV6Addr() (ipv6 []*net.IP, err error) {
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
		ipv6 = append(ipv6, &localAddr.IP)
		if localAddr.IP.To16() != nil {
			break
		}
	}
	return
}
