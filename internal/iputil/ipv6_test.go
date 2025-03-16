package iputil_test

import (
	"context"
	"fmt"

	"github.com/notes-bin/ddns6/internal/iputil"
)

func ExmapleGetIPv6Addr() {
	// 获取 IPv6 地址
	ip, err := iputil.GetIPv6Addr(context.Background())
	if err != nil {
		fmt.Println("Failed to get IPv6 address:", err)
		return
	}

	fmt.Println("IPv6 address:", ip)
}
