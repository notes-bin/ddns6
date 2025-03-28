package iputil_test

import (
	"context"
	"testing"
	"time"

	"github.com/notes-bin/ddns6/internal/iputil"
)

func TestGetIPv6Addr(t *testing.T) {
	provider := iputil.NewMultiProvider(
		iputil.NewSiteProvider(),
		iputil.NewDNSProvider(),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ip, err := provider.GetIPv6Addr(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ip)
}
