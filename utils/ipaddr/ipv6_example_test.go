package ipaddr_test

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/notes-bin/ddns6/utils/ipaddr"
)

func ExmapleGetIPv6Addr() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// 创建多源提供者
	provider := ipaddr.NewMultiProvider(
		ipaddr.NewSiteProvider(),
		ipaddr.NewDNSProvider(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ip, err := provider.GetIPv6Addr(ctx)
	if err != nil {
		slog.Error("failed to get IPv6 address", "error", err)
		return
	}

	slog.Info("got IPv6 address", "ip", ip)
}
