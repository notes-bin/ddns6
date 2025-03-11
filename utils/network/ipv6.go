// get ipv6 address
// use three methods, from network card, ipv6 website, dns server
package network

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
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
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
	return &site{
		urls: urls,
	}
}

func (s *site) GetIPV6Addr() (ipv6 []*net.IP, err error) {
	for _, url := range s.urls {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()
		res, err := io.ReadAll(resp.Body)
		if err != nil {
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
	return &publicDns{
		port: 53,
		dns:  dns,
	}

}

func (p *publicDns) GetIPV6Addr() (ipv6 []*net.IP, err error) {
	i := 0
	// 连接到一个IPv6的DNS服务器，例如Google的公共DNS服务器
	for _, ip := range p.dns {
		conn, err := net.Dial("udp6", fmt.Sprintf("[%s]:%d", ip, p.port))
		if err != nil {
			i++
			if i == len(p.dns) {
				return nil, err
			}
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
