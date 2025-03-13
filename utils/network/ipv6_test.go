package network

import (
	"os"
	"testing"
)

func TestGetIPV6Addr(t *testing.T) {
	os.Setenv("IFACE", "en0")
	i := NewIface()
	ipv6, err := i.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	t.Logf("ipv6 -> %v\n", ipv6)
}

func TestGetIPV6AddrBySite(t *testing.T) {
	os.Setenv("SITES", "https://6.ipw.cn")
	s := NewSite()
	ipv6, err := s.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	t.Logf("ipv6 -> %v\n", ipv6)
}

func TestGetIPV6AddrByPublicDns(t *testing.T) {
	os.Setenv("PUBLIC_DNS", "2402:4e00::")
	p := NewPublicDNS()
	ipv6, err := p.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	t.Logf("ipv6 -> %v\n", ipv6)
}
