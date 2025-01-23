package utils

import (
	"fmt"
	"testing"
)

func TestGetIPV6Addr(t *testing.T) {
	i := NewIface("en0")
	err := i.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(i.ipv6)
}

func TestGetIPV6AddrBySite(t *testing.T) {
	s := NewSite("https://6.ipw.cn")
	err := s.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(s.ipv6)
}

func TestGetIPV6AddrByPublicDns(t *testing.T) {
	p := NewPublicDNS("2402:4e00::")
	err := p.GetIPV6Addr()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(p.ipv6)
}
