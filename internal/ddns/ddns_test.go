package ddns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
)

// TestRecordNameMatches 覆盖各服务商常见记录名格式的匹配与否。
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

// TestIPv6Equal 验证 IP 解析比较与无效字符串回退行为。
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

// TestHasAddressChanged_NilCached 验证无缓存时视为地址已变化。
func TestHasAddressChanged_NilCached(t *testing.T) {
	addr := net.ParseIP("::1")
	if !hasAddressChanged(nil, addr) {
		t.Error("nil cached address should be considered changed")
	}
}

// TestHasAddressChanged_Same 验证相同地址不视为变化。
func TestHasAddressChanged_Same(t *testing.T) {
	addr := net.ParseIP("::1")
	if hasAddressChanged(addr, addr) {
		t.Error("same address should not be considered changed")
	}
}

// TestHasAddressChanged_Different 验证不同地址视为已变化。
func TestHasAddressChanged_Different(t *testing.T) {
	old := net.ParseIP("::1")
	new := net.ParseIP("::2")
	if !hasAddressChanged(old, new) {
		t.Error("different addresses should be considered changed")
	}
}

// TestRecordInfoKey_Basic 验证 Key 的拼接格式。
func TestRecordInfoKey_Basic(t *testing.T) {
	r := RecordInfo{ID: "123", Name: "www", Type: "AAAA", Value: "::1", TTL: 600}
	expected := "123|www|AAAA|::1"
	if got := r.Key(); got != expected {
		t.Errorf("Key() = %q, want %q", got, expected)
	}
}

// TestRecordInfoKey_EmptyID 验证空 ID 时 Key 仍可区分记录。
func TestRecordInfoKey_EmptyID(t *testing.T) {
	r := RecordInfo{ID: "", Name: "www", Type: "AAAA", Value: "::1"}
	expected := "|www|AAAA|::1"
	if got := r.Key(); got != expected {
		t.Errorf("Key() with empty ID = %q, want %q", got, expected)
	}
}

// mockProvider 实现 DNSProvider，供 syncDNSRecord / CollectMatchingRecords 测试使用。
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

// TestSyncDNSRecord_NoRecord_AddNew 验证无记录时新增并更新本地缓存。
func TestSyncDNSRecord_NoRecord_AddNew(t *testing.T) {
	ctx := t.Context()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	m := &mockProvider{records: []RecordInfo{}}
	addr := net.ParseIP("2001:db8::1")

	err := syncDNSRecord(ctx, d, m, addr)
	if err != nil {
		t.Fatalf("syncDNSRecord 不应返回错误: %v", err)
	}

	if d.Addr == nil || d.Addr.String() != "2001:db8::1" {
		t.Errorf("Addr 应更新为 2001:db8::1, 得到 %v", d.Addr)
	}
}

// TestSyncDNSRecord_IPMatch_Skip 验证 IP 已一致时跳过修改但仍刷新缓存。
func TestSyncDNSRecord_IPMatch_Skip(t *testing.T) {
	ctx := t.Context()
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

	if d.Addr == nil || d.Addr.String() != "2001:db8::1" {
		t.Errorf("Addr 应更新为 2001:db8::1, 得到 %v", d.Addr)
	}
}

// TestSyncDNSRecord_IPChanged_Modify 验证 IP 变化时修改记录并更新缓存。
func TestSyncDNSRecord_IPChanged_Modify(t *testing.T) {
	ctx := t.Context()
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

// TestSyncDNSRecord_GetRecordsError 验证查询失败时向上返回错误。
func TestSyncDNSRecord_GetRecordsError(t *testing.T) {
	ctx := t.Context()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	m := &mockProvider{getErr: fmt.Errorf("api failure")}
	addr := net.ParseIP("2001:db8::1")

	err := syncDNSRecord(ctx, d, m, addr)
	if err == nil {
		t.Fatal("GetRecords 失败时 syncDNSRecord 应返回错误")
	}
}

// TestSyncDNSRecord_ModifyRecordError 验证修改失败时向上返回错误。
func TestSyncDNSRecord_ModifyRecordError(t *testing.T) {
	ctx := t.Context()
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

// TestSyncDNSRecord_AddRecordError 验证新增失败时向上返回错误。
func TestSyncDNSRecord_AddRecordError(t *testing.T) {
	ctx := t.Context()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	m := &mockProvider{addErr: fmt.Errorf("add failed")}
	addr := net.ParseIP("2001:db8::1")

	err := syncDNSRecord(ctx, d, m, addr)
	if err == nil {
		t.Fatal("AddRecord 失败时 syncDNSRecord 应返回错误")
	}
}

// TestSyncDNSRecord_MultipleRecords_AllProcessed 验证同名多条记录均被处理。
func TestSyncDNSRecord_MultipleRecords_AllProcessed(t *testing.T) {
	ctx := t.Context()
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

// TestSyncDNSRecord_WrongType_Skipped 验证类型不符的记录被跳过并触发新增。
func TestSyncDNSRecord_WrongType_Skipped(t *testing.T) {
	ctx := t.Context()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	addr := net.ParseIP("2001:db8::1")
	m := &mockProvider{
		records: []RecordInfo{
			{ID: "1", Name: "www.example.com", Zone: "example.com", Type: "A", Value: "192.168.1.1", TTL: 600},
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

// TestSyncDNSRecord_WrongSubDomain_Skipped 验证子域名不符的记录被跳过并触发新增。
func TestSyncDNSRecord_WrongSubDomain_Skipped(t *testing.T) {
	ctx := t.Context()
	d := &Domain{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600}
	addr := net.ParseIP("2001:db8::1")
	m := &mockProvider{
		records: []RecordInfo{
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

// TestSyncRecord_AddrUnchanged_Skip 验证地址未变时 SyncRecord 跳过 API。
func TestSyncRecord_AddrUnchanged_Skip(t *testing.T) {
	ctx := t.Context()
	addr := net.ParseIP("2001:db8::1")
	d := &Domain{
		Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600,
	}
	d.CheckAndSetAddr(addr)

	m := &mockProvider{records: []RecordInfo{}}

	err := SyncRecord(ctx, d, addr, m)
	if err != nil {
		t.Fatalf("SyncRecord 不应返回错误: %v", err)
	}
}

// TestSyncRecord_AddrChanged_Update 验证地址变化时 SyncRecord 更新 DNS 与缓存。
func TestSyncRecord_AddrChanged_Update(t *testing.T) {
	ctx := t.Context()
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

	if d.Addr.String() != "2001:db8::2" {
		t.Errorf("Addr 应更新为 2001:db8::2, 得到 %v", d.Addr)
	}
}

// TestSyncRecord_NilCachedAddr_Update 验证首次运行（缓存为 nil）时会同步。
func TestSyncRecord_NilCachedAddr_Update(t *testing.T) {
	ctx := t.Context()
	addr := net.ParseIP("2001:db8::1")
	d := &Domain{
		Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600,
	}

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

// TestSyncRecord_CtxCancelled 验证上下文已取消时 SyncRecord 立即返回错误。
func TestSyncRecord_CtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

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

// TestCollectMatchingRecords 覆盖子域名过滤、不去重过滤、去重与查询错误。
func TestCollectMatchingRecords(t *testing.T) {
	dup := RecordInfo{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1"}
	domains := []*Domain{{Domain: "example.com", SubDomain: "www", Type: "AAAA"}}

	tests := []struct {
		name        string
		records     []RecordInfo
		getErr      error
		filter      bool
		wantIDs     []string
		errContains string
	}{
		{
			name: "按子域名过滤",
			records: []RecordInfo{
				{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1"},
				{ID: "2", Name: "api.example.com", Type: "AAAA", Value: "2001:db8::2"},
				{ID: "3", Name: "www.example.com", Type: "A", Value: "1.2.3.4"},
			},
			filter:  true,
			wantIDs: []string{"1"},
		},
		{
			name: "不过滤子域名",
			records: []RecordInfo{
				{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1"},
				{ID: "2", Name: "api.example.com", Type: "AAAA", Value: "2001:db8::2"},
			},
			wantIDs: []string{"1", "2"},
		},
		{
			name:    "去重",
			records: []RecordInfo{dup, dup},
			wantIDs: []string{"1"},
		},
		{
			name:        "查询错误",
			getErr:      fmt.Errorf("boom"),
			filter:      true,
			errContains: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CollectMatchingRecords(t.Context(), &mockProvider{records: tt.records, getErr: tt.getErr}, domains, "AAAA", tt.filter)
			if tt.errContains != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error should include %q: %v", tt.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("got %d records %+v, want IDs %v", len(got), got, tt.wantIDs)
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}
