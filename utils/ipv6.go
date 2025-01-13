package utils

import (
	"fmt"
	"io"
	"net"
	"net/http"
)

func GetIPV6AddrByInterface(ifaceName string) {
	iface, err := net.InterfaceByName(ifaceName)
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
			fmt.Printf("IPv6 Address on %s: %s\n", ifaceName, ip)
		}
	}

}

func GetIPV6AddrBySite() {
	resp, err := http.Get("https://6.ipw.cn")
	if err != nil {
		fmt.Println("request err -> ", err)
		return
	}
	defer resp.Body.Close()
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("read err -> ", err)
		return
	}
	fmt.Printf("val: %v", string(res))
}

func GetIPV6AddrByDNS() {
	// 连接到一个IPv6的DNS服务器，例如Google的公共DNS服务器
	dnsServer := "[2001:4860:4860::8888]:53"
	// dnsServer := "[2400:3200:baba::1]:53"
	conn, err := net.Dial("udp6", dnsServer)
	if err != nil {
		fmt.Println("Error connecting to DNS server:", err)
		return
	}
	defer conn.Close()

	// 获取本机的IPv6地址
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP
	fmt.Println("Local IPv6 Address:", ip)
}
