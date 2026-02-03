package ipaddr_test

import (
	"context"
	"fmt"
	"time"

	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

func ExampleHttpIPv6Fetcher() {
	url := "https://ipv6.icanhazip.com"
	fetcher := ipaddr.NewHttpIPv6Fetcher(url)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ip, err := fetcher.Fetch(ctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("IPv6 Address:", ip)
}

func ExampleDnsFetcher() {
	dnsServer := "2001:4860:4860::8888" // Google DNS
	fetcher := ipaddr.NewDnsFetcher(dnsServer)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ip, err := fetcher.Fetch(ctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("IPv6 Address:", ip)
}
