package utils

import (
	"testing"
)

func TestGetIPV6Addr(t *testing.T) {
	i := NewIface("en0")
	ipv6, err := i.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	t.Logf("ipv6 -> %v\n", ipv6)
}

func TestGetIPV6AddrBySite(t *testing.T) {
	s := NewSite("https://6.ipw.cn")
	ipv6, err := s.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	t.Logf("ipv6 -> %v\n", ipv6)
}

func TestGetIPV6AddrByPublicDns(t *testing.T) {
	p := NewPublicDNS("2402:4e00::")
	ipv6, err := p.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	t.Logf("ipv6 -> %v\n", ipv6)
}
