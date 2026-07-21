package ddns

import (
	"context"
	"net"
	"testing"
)

// ============================================================
// RecordNameMatches 测试
// ============================================================

func TestRecordNameMatches_FullDomain(t *testing.T) {
	// 完整域名匹配
	if !RecordNameMatches("www.example.com", "www.example.com", "www") {
		t.Error("should match full domain")
	}
}

func TestRecordNameMatches_FullDomainWithDot(t *testing.T) {
	// 带尾部点号的完整域名
	if !RecordNameMatches("www.example.com.", "www.example.com", "www") {
		t.Error("should match full domain with trailing dot")
	}
}

func TestRecordNameMatches_SubDomainLabel(t *testing.T) {
	// 仅子域名标签
	if !RecordNameMatches("www", "www.example.com", "www") {
		t.Error("should match subdomain label")
	}
}

func TestRecordNameMatches_RootDomainAt(t *testing.T) {
	// 根域名 @ 标签
	if !RecordNameMatches("@", "example.com", "@") {
		t.Error("should match @ for root domain")
	}
}

func TestRecordNameMatches_RootDomainEmpty(t *testing.T) {
	// 根域名空字符串
	if !RecordNameMatches("", "example.com", "@") {
		t.Error("should match empty string for root domain")
	}
}

func TestRecordNameMatches_NoMatch(t *testing.T) {
	// 不匹配
	if RecordNameMatches("api", "www.example.com", "www") {
		t.Error("should not match different subdomain")
	}
}

func TestRecordNameMatches_RootDomainWithDifferentFQDN(t *testing.T) {
	// 根域名带点号
	if !RecordNameMatches("example.com.", "example.com", "@") {
		t.Error("should match root domain with trailing dot")
	}
}

// ============================================================
// ipv6Equal 测试
// ============================================================

func TestIPv6Equal_Same(t *testing.T) {
	if !ipv6Equal("::1", "::1") {
		t.Error("same IPv6 should be equal")
	}
}

func TestIPv6Equal_DifferentFormat(t *testing.T) {
	// 不同格式的同一地址
	if !ipv6Equal("2001:db8::1", "2001:0db8:0000:0000:0000:0000:0000:0001") {
		t.Error("same IPv6 in different formats should be equal")
	}
}

func TestIPv6Equal_Different(t *testing.T) {
	if ipv6Equal("::1", "::2") {
		t.Error("different IPv6 should not be equal")
	}
}

func TestIPv6Equal_InvalidFallback(t *testing.T) {
	// 无效地址回退到字符串比较
	if !ipv6Equal("invalid-ip", "invalid-ip") {
		t.Error("invalid IPs should fall back to string comparison")
	}
}

func TestIPv6Equal_InvalidDifferent(t *testing.T) {
	if ipv6Equal("invalid-1", "invalid-2") {
		t.Error("different invalid IPs should not match")
	}
}

// ============================================================
// hasAddressChanged 测试
// ============================================================

func TestHasAddressChanged_NilCached(t *testing.T) {
	// 缓存为空 → 视为变化
	addr := net.ParseIP("::1")
	if !hasAddressChanged(nil, addr) {
		t.Error("nil cached address should be considered changed")
	}
}

func TestHasAddressChanged_Same(t *testing.T) {
	addr := net.ParseIP("::1")
	if hasAddressChanged(addr, addr) {
		t.Error("same address should not be considered changed")
	}
}

func TestHasAddressChanged_Different(t *testing.T) {
	old := net.ParseIP("::1")
	new := net.ParseIP("::2")
	if !hasAddressChanged(old, new) {
		t.Error("different addresses should be considered changed")
	}
}

// ============================================================
// RecordInfo.Key 测试
// ============================================================

func TestRecordInfoKey_Basic(t *testing.T) {
	r := RecordInfo{ID: "123", Name: "www", Type: "AAAA", Value: "::1", TTL: 600}
	expected := "123|www|AAAA|::1"
	if got := r.Key(); got != expected {
		t.Errorf("Key() = %q, want %q", got, expected)
	}
}

func TestRecordInfoKey_EmptyID(t *testing.T) {
	r := RecordInfo{ID: "", Name: "www", Type: "AAAA", Value: "::1"}
	expected := "|www|AAAA|::1"
	if got := r.Key(); got != expected {
		t.Errorf("Key() with empty ID = %q, want %q", got, expected)
	}
}

// ============================================================
// Mock DNSProvider — 供后续 syncDNSRecord 测试使用
// ============================================================

// mockProvider 实现 DNSProvider 接口，用于测试。
type mockProvider struct {
	records []RecordInfo
	addErr  error
	modErr  error
	getErr  error
}

func (m *mockProvider) GetRecords(_ context.Context, _, _ string) ([]RecordInfo, error) {
	return m.records, m.getErr
}

func (m *mockProvider) AddRecord(_ context.Context, _ RecordInfo) error {
	return m.addErr
}

func (m *mockProvider) ModifyRecord(_ context.Context, _ RecordInfo) error {
	return m.modErr
}

func (m *mockProvider) DeleteRecord(_ context.Context, _ RecordInfo) error {
	return nil
}
