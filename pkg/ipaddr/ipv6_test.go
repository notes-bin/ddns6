package ipaddr_test

import (
	"testing"

	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

func TestGetIPv6Addr(t *testing.T) {
	req := ipaddr.New()
	res, err := req.GetIPv6Addr(t.Context())
	t.Logf("ipaddr: %v, err: %v\n", res, err)
}
