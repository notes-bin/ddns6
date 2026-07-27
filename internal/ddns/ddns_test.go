package ddns

import (
	"context"
	"fmt"
	"net"
	"testing"
)

// ============================================================
// RecordNameMatches 测试
// ============================================================

func TestRecordNameMatches(t *testing.T) {
	tests := []struct {
		name      string
		record    string
		fqdn      string
		subdomain string
		want      bool
	}{
		{name: "完整域名匹配", record: "www.example.com", fqdn: "www.example.com", subdomain: "www", want: true},
		{name: "带尾部点号的完整域名", record: "www.example.com.", fqdn: "www.example.com", subdomain: "www", want: true},
		{name: "仅子域名标签", record: "www", fqdn: "www.example.com", subdomain: "www", want: true},
		{name: "根域名 @ 标签", record: "@", fqdn: "example.com", subdomain: "@", want: true},
		{name: "根域名空字符串", record: "", fqdn: "example.com", subdomain: "@", want: true},
		{name: "不匹配", record: "api", fqdn: "www.example.com", subdomain: "www", want: false},
		{name: "根域名带点号", record: "example.com.", fqdn: "example.com", subdomain: "@", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecordNameMatches(tt.record, tt.fqdn, tt.subdomain)
			if got != tt.want {
				t.Errorf("RecordNameMatches(%q, %q, %q) = %v, want %v", tt.record, tt.fqdn, tt.subdomain, got, tt.want)
			}
		})
	}
}

// ============================================================
// ipv6Equal 测试
// ============================================================

func TestIPv6Equal(t *testing.T) {
	tests := []struct {
		name string
		a    net.IP
		b    string
		want bool
	}{
		{name: "相同地址", a: net.ParseIP("::1"), b: "::1", want: true},
		{name: "不同格式相同地址", a: net.ParseIP("2001:db8::1"), b: "2001:0db8:0000:0000:0000:0000:0000:0001", want: true},
		{name: "不同地址", a: net.ParseIP("::1"), b: "::2", want: false},
		{name: "nil IP与无效字符串不匹配", a: nil, b: "invalid-ip", want: false},
		{name: "不同无效地址", a: nil, b: "invalid-1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ipv6Equal(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("ipv6Equal(%v, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ============================================================
// hasAddressChanged 测试
// ============================================================

func TestHasAddressChanged_NilCached(t *testing.T) {
	// 缓存为空 -> 视为变化
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
// Mock DNSProvider - 供后续 syncDNSRecord 测试使用
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

// ============================================================
// syncDNSRecord 测试
// ============================================================

func TestSyncDNSRecord_NoRecord_AddNew(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	m := &mockProvider{records: []RecordInfo{}}
	addr := net.ParseIP("2001:db8::1")

	err := syncDNSRecord(ctx, d, m, addr)
	if err != nil {
		t.Fatalf("syncDNSRecord 不应返回错误: %v", err)
	}

	// 验证缓存已更新
	if d.Addr == nil || d.Addr.String() != "2001:db8::1" {
		t.Errorf("Addr 应更新为 2001:db8::1, 得到 %v", d.Addr)
	}
}

func TestSyncDNSRecord_IPMatch_Skip(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	addr := net.ParseIP("2001:db8::1")
	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
	}

	err := syncDNSRecord(ctx, d, m, addr)
	if err != nil {
		t.Fatalf("syncDNSRecord 不应返回错误: %v", err)
	}

	// 验证缓存已更新（IP 不变时也应更新缓存）
	if d.Addr == nil || d.Addr.String() != "2001:db8::1" {
		t.Errorf("Addr 应更新为 2001:db8::1, 得到 %v", d.Addr)
	}
}

func TestSyncDNSRecord_IPChanged_Modify(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	addr := net.ParseIP("2001:db8::2")
	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
	}

	err := syncDNSRecord(ctx, d, m, addr)
	if err != nil {
		t.Fatalf("syncDNSRecord 不应返回错误: %v", err)
	}

	if d.Addr == nil || d.Addr.String() != "2001:db8::2" {
		t.Errorf("Addr 应更新为 2001:db8::2, 得到 %v", d.Addr)
	}
}

func TestSyncDNSRecord_GetRecordsError(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	m := &mockProvider{getErr: fmt.Errorf("api failure")}
	addr := net.ParseIP("2001:db8::1")

	err := syncDNSRecord(ctx, d, m, addr)
	if err == nil {
		t.Fatal("GetRecords 失败时 syncDNSRecord 应返回错误")
	}
}

func TestSyncDNSRecord_ModifyRecordError(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	addr := net.ParseIP("2001:db8::2")
	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
		modErr: fmt.Errorf("modify failed"),
	}

	err := syncDNSRecord(ctx, d, m, addr)
	if err == nil {
		t.Fatal("ModifyRecord 失败时 syncDNSRecord 应返回错误")
	}
}

func TestSyncDNSRecord_AddRecordError(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	m := &mockProvider{addErr: fmt.Errorf("add failed")}
	addr := net.ParseIP("2001:db8::1")

	err := syncDNSRecord(ctx, d, m, addr)
	if err == nil {
		t.Fatal("AddRecord 失败时 syncDNSRecord 应返回错误")
	}
}

func TestSyncDNSRecord_MultipleRecords_AllProcessed(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	addr := net.ParseIP("2001:db8::3")
	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
			{ID: "2", Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::2", TTL: 600},
		},
	}

	err := syncDNSRecord(ctx, d, m, addr)
	if err != nil {
		t.Fatalf("syncDNSRecord 不应返回错误: %v", err)
	}

	if d.Addr == nil || d.Addr.String() != "2001:db8::3" {
		t.Errorf("Addr 应更新为 2001:db8::3, 得到 %v", d.Addr)
	}
}

func TestSyncDNSRecord_WrongType_Skipped(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	addr := net.ParseIP("2001:db8::1")
	m := &mockProvider{
		records: []RecordInfo{
			// 非 AAAA 类型记录应被跳过
			{ID: "1", Name: "www.example.com", Zone: "example.com", Type: "A", Value: "192.168.1.1", TTL: 600},
		},
	}

	err := syncDNSRecord(ctx, d, m, addr)
	if err != nil {
		t.Fatalf("syncDNSRecord 不应返回错误: %v", err)
	}

	// 无匹配记录时应触发 Add
	if d.Addr == nil || d.Addr.String() != "2001:db8::1" {
		t.Errorf("Addr 应更新为 2001:db8::1, 得到 %v", d.Addr)
	}
}

func TestSyncDNSRecord_WrongSubDomain_Skipped(t *testing.T) {
	ctx := context.Background()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	addr := net.ParseIP("2001:db8::1")
	m := &mockProvider{
		records: []RecordInfo{
			// 非 www 子域名的记录应被跳过
			{ID: "1", Name: "api.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
	}

	err := syncDNSRecord(ctx, d, m, addr)
	if err != nil {
		t.Fatalf("syncDNSRecord 不应返回错误: %v", err)
	}

	if d.Addr == nil || d.Addr.String() != "2001:db8::1" {
		t.Errorf("Addr 应更新为 2001:db8::1, 得到 %v", d.Addr)
	}
}

// ============================================================
// SyncRecord 测试
// ============================================================

func TestSyncRecord_AddrUnchanged_Skip(t *testing.T) {
	ctx := context.Background()
	addr := net.ParseIP("2001:db8::1")
	d := &Domain{
		Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600,
	}
	d.CheckAndSetAddr(addr) // 缓存已是最新

	m := &mockProvider{records: []RecordInfo{}}

	err := SyncRecord(ctx, d, addr, m)
	if err != nil {
		t.Fatalf("SyncRecord 不应返回错误: %v", err)
	}
	// 地址未变化时应跳过 API 调用
}

func TestSyncRecord_AddrChanged_Update(t *testing.T) {
	ctx := context.Background()
	oldAddr := net.ParseIP("2001:db8::1")
	newAddr := net.ParseIP("2001:db8::2")
	d := &Domain{
		Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600,
	}
	d.CheckAndSetAddr(oldAddr)

	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
	}

	err := SyncRecord(ctx, d, newAddr, m)
	if err != nil {
		t.Fatalf("SyncRecord 不应返回错误: %v", err)
	}

	// 地址变化应更新
	if d.Addr.String() != "2001:db8::2" {
		t.Errorf("Addr 应更新为 2001:db8::2, 得到 %v", d.Addr)
	}
}

func TestSyncRecord_NilCachedAddr_Update(t *testing.T) {
	ctx := context.Background()
	addr := net.ParseIP("2001:db8::1")
	d := &Domain{
		Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600,
	}
	// Addr 为 nil - 首次运行

	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
		},
	}

	err := SyncRecord(ctx, d, addr, m)
	if err != nil {
		t.Fatalf("SyncRecord 不应返回错误: %v", err)
	}
}

func TestSyncRecord_CtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	addr := net.ParseIP("2001:db8::1")
	d := &Domain{
		Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600,
	}
	m := &mockProvider{}

	err := SyncRecord(ctx, d, addr, m)
	if err == nil {
		t.Fatal("上下文取消时 SyncRecord 应返回错误")
	}
}
