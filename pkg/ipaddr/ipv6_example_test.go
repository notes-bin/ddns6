package ipaddr_test

import (
	"context"
	"fmt"
	"time"

	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// ExampleHTTPIPv6Fetcher 演示通过 HTTP 端点获取本机公网 IPv6。
func ExampleHTTPIPv6Fetcher() {
	url := "https://ipv6.icanhazip.com"
	fetcher := ipaddr.NewHTTPIPv6Fetcher(url)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ip, err := fetcher.Fetch(ctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("IPv6 Address:", ip)
}

// ExampleDNSFetcher 演示通过 UDP6 拨号 DNS 获取本机 IPv6。
func ExampleDNSFetcher() {
	dnsServer := "2001:4860:4860::8888" // Google DNS
	fetcher := ipaddr.NewDNSFetcher(dnsServer)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ip, err := fetcher.Fetch(ctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("IPv6 Address:", ip)
}
