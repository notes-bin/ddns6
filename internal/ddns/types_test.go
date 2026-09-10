package ddns

import (
	"net"
	"strings"
	"testing"
)

// TestDomainFullDomain 覆盖空子域名、@ 与普通子域名三种拼接。
func TestDomainFullDomain(t *testing.T) {
	tests := []struct {
		name      string
		domain    string
		subDomain string
		want      string
	}{
		{name: "普通子域名", domain: "example.com", subDomain: "www", want: "www.example.com"},
		{name: "根域名@", domain: "example.com", subDomain: "@", want: "example.com"},
		{name: "空子域名", domain: "example.com", subDomain: "", want: "example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Domain{Domain: tt.domain, SubDomain: tt.subDomain}
			if got := d.FullDomain(); got != tt.want {
				t.Errorf("FullDomain() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDomainStringAndAddrString 验证 String / AddrString 线程安全读出缓存地址。
func TestDomainStringAndAddrString(t *testing.T) {
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA"}
	if got := d.AddrString(); got != "<nil>" {
		t.Errorf("空 Addr 时 AddrString = %q, want <nil>", got)
	}

	addr := net.ParseIP("2001:db8::1")
	d.CheckAndSetAddr(addr)
	if got := d.AddrString(); got != "2001:db8::1" {
		t.Errorf("AddrString = %q, want 2001:db8::1", got)
	}

	s := d.String()
	for _, want := range []string{"www.example.com", "2001:db8::1", "AAAA"} {
		if !strings.Contains(s, want) {
			t.Errorf("String 应包含 %q, got %q", want, s)
		}
	}
}

// TestCheckAndSetAddr 覆盖首次设置、相同跳过、变化更新。
func TestCheckAndSetAddr(t *testing.T) {
	d := &Domain{}
	a1 := net.ParseIP("2001:db8::1")
	a2 := net.ParseIP("2001:db8::2")

	if !d.CheckAndSetAddr(a1) {
		t.Error("首次设置应返回 true")
	}
	if d.CheckAndSetAddr(a1) {
		t.Error("相同地址应返回 false")
	}
	if !d.CheckAndSetAddr(a2) {
		t.Error("地址变化应返回 true")
	}
	if got := d.AddrString(); got != "2001:db8::2" {
		t.Errorf("Addr = %q, want 2001:db8::2", got)
	}
}

// TestIPv6Equal_ValidIPVersusInvalidString 覆盖非 nil IP 与无法解析字符串的回退比较。
func TestIPv6Equal_ValidIPVersusInvalidString(t *testing.T) {
	a := net.ParseIP("::1")
	if ipv6Equal(a, "not-an-ip") {
		t.Error("有效 IP 与无效字符串不应相等")
	}
}

// TestIPv6Equal_NilStringFallback 覆盖 ParseIP 失败时与 a.String() 的相等回退。
func TestIPv6Equal_NilStringFallback(t *testing.T) {
	if !ipv6Equal(nil, "<nil>") {
		t.Error(`nil IP 与 "<nil>" 字符串应视为相等`)
	}
}
