//go:build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/notes-bin/ddns6/internal/ddns"
)

// ============================================================
// Sync 流程集成测试
//
// 使用 mock DNSProvider 验证完整的同步流程：
//   - IP 未变化时跳过更新
//   - IP 变化时修改记录
//   - 无现有记录时创建新记录
//   - 多子域名同步
// ============================================================

// mockProvider 实现 ddns.DNSProvider 接口
type mockProvider struct {
	mu            sync.Mutex
	records       []ddns.RecordInfo
	addCalls      int
	modifyCalls   int
	deleteCalls   int
	queryCalls    int
	failQuery     bool
	failAdd       bool
	failModify    bool
	failDelete    bool
	addRecordHook func(domain, recordType, value string)
	modifyHook    func(recordID, newValue string)
}

func newMockProvider() *mockProvider {
	return &mockProvider{}
}

func (m *mockProvider) AddRecord(ctx context.Context, record ddns.RecordInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addCalls++
	if m.failAdd {
		return fmt.Errorf("mock add error")
	}
	if m.addRecordHook != nil {
		m.addRecordHook(record.Name, record.Type, record.Value)
	}
	m.records = append(m.records, ddns.RecordInfo{
		ID:    "new-" + record.Name,
		Name:  record.Name,
		Type:  record.Type,
		Value: record.Value,
		TTL:   record.TTL,
	})
	return nil
}

func (m *mockProvider) ModifyRecord(ctx context.Context, record ddns.RecordInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modifyCalls++
	if m.failModify {
		return fmt.Errorf("mock modify error")
	}
	if m.modifyHook != nil {
		m.modifyHook(record.ID, record.Value)
	}
	for i := range m.records {
		if m.records[i].ID == record.ID {
			m.records[i].Value = record.Value
			break
		}
	}
	return nil
}

func (m *mockProvider) DeleteRecord(ctx context.Context, record ddns.RecordInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls++
	if m.failDelete {
		return fmt.Errorf("mock delete error")
	}
	for i := range m.records {
		if m.records[i].ID == record.ID {
			m.records = append(m.records[:i], m.records[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockProvider) GetRecords(ctx context.Context, domain, recordType string) ([]ddns.RecordInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCalls++
	if m.failQuery {
		return nil, fmt.Errorf("mock query error")
	}
	// 返回所有匹配的记录
	var result []ddns.RecordInfo
	for _, r := range m.records {
		if r.Type == recordType {
			result = append(result, r)
		}
	}
	if result == nil {
		return []ddns.RecordInfo{}, nil
	}
	return result, nil
}

// TestSyncIPUnchanged 验证 IP 未变化时跳过更新
func TestSyncIPUnchanged(t *testing.T) {
	mock := newMockProvider()
	ip := net.ParseIP("2001:db8::1")

	d := &ddns.Domain{
		Domain:    "example.com",
		SubDomain: "www",
		Type:      "AAAA",
		TTL:       600,
	}
	d.SetAddr(ip) // 设置缓存地址，与当前 IP 相同

	err := ddns.SyncRecord(context.Background(), d, ip, mock)
	if err != nil {
		t.Fatalf("SyncRecord failed: %v", err)
	}
	if mock.queryCalls > 0 {
		t.Errorf("expected no query calls when IP unchanged, got %d", mock.queryCalls)
	}
	if mock.modifyCalls > 0 {
		t.Errorf("expected no modify calls when IP unchanged, got %d", mock.modifyCalls)
	}
}

// TestSyncIPChanged 验证 IP 变化时修改现有记录
func TestSyncIPChanged(t *testing.T) {
	mock := newMockProvider()

	// 预先存在一条记录
	mock.records = []ddns.RecordInfo{
		{ID: "rec1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1", TTL: 600},
	}

	oldIP := net.ParseIP("2001:db8::1") // 缓存的旧 IP
	newIP := net.ParseIP("2001:db8::2") // 当前新 IP

	d := &ddns.Domain{
		Domain:    "example.com",
		SubDomain: "www",
		Type:      "AAAA",
		TTL:       600,
	}
	d.SetAddr(oldIP)

	// 第一次同步：记录已有，IP 不同 → 应修改
	err := ddns.SyncRecord(context.Background(), d, newIP, mock)
	if err != nil {
		t.Fatalf("SyncRecord failed: %v", err)
	}
	if mock.modifyCalls != 1 {
		t.Errorf("expected 1 modify call, got %d", mock.modifyCalls)
	}
	if mock.addCalls != 0 {
		t.Errorf("expected 0 add calls, got %d", mock.addCalls)
	}

	// 验证记录值已更新
	if mock.records[0].Value != "2001:db8::2" {
		t.Errorf("expected record value to be updated to 2001:db8::2, got %s", mock.records[0].Value)
	}
}

// TestSyncNoExistingRecord 验证无现有记录时自动创建
func TestSyncNoExistingRecord(t *testing.T) {
	mock := newMockProvider()
	ip := net.ParseIP("2001:db8::1")

	d := &ddns.Domain{
		Domain:    "example.com",
		SubDomain: "www",
		Type:      "AAAA",
		TTL:       600,
	}

	err := ddns.SyncRecord(context.Background(), d, ip, mock)
	if err != nil {
		t.Fatalf("SyncRecord failed: %v", err)
	}
	if mock.addCalls != 1 {
		t.Errorf("expected 1 add call, got %d", mock.addCalls)
	}
	if mock.modifyCalls != 0 {
		t.Errorf("expected 0 modify calls, got %d", mock.modifyCalls)
	}
	if mock.queryCalls != 1 {
		t.Errorf("expected 1 query call, got %d", mock.queryCalls)
	}

	// 验证缓存已更新
	if d.Addr == nil || !d.Addr.Equal(ip) {
		t.Error("Domain.Addr should be updated after sync")
	}
}

// TestSyncMultipleDomains 验证多子域名同步
func TestSyncMultipleDomains(t *testing.T) {
	mock := newMockProvider()
	ip := net.ParseIP("2001:db8::1")

	domains := []*ddns.Domain{
		{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600},
		{Domain: "example.com", SubDomain: "@", Type: "AAAA", TTL: 600},
		{Domain: "example.com", SubDomain: "api", Type: "AAAA", TTL: 600},
	}

	for _, d := range domains {
		err := ddns.SyncRecord(context.Background(), d, ip, mock)
		if err != nil {
			t.Fatalf("SyncRecord failed for %s: %v", d.SubDomain, err)
		}
	}

	if mock.addCalls != 3 {
		t.Errorf("expected 3 add calls (one per subdomain), got %d", mock.addCalls)
	}
}

// TestSyncConcurrent 验证并发同步的安全性
func TestSyncConcurrent(t *testing.T) {
	mock := newMockProvider()
	ip := net.ParseIP("2001:db8::1")

	domains := []*ddns.Domain{
		{Domain: "example.com", SubDomain: "www", Type: "AAAA", TTL: 600},
		{Domain: "example.com", SubDomain: "api", Type: "AAAA", TTL: 600},
		{Domain: "example.com", SubDomain: "mail", Type: "AAAA", TTL: 600},
	}

	var wg sync.WaitGroup
	for _, d := range domains {
		wg.Add(1)
		go func(dom *ddns.Domain) {
			defer wg.Done()
			if err := ddns.SyncRecord(context.Background(), dom, ip, mock); err != nil {
				t.Errorf("SyncRecord failed for %s: %v", dom.SubDomain, err)
			}
		}(d)
	}
	wg.Wait()

	totalCalls := mock.addCalls + mock.modifyCalls
	if totalCalls != 3 {
		t.Errorf("expected 3 total operations, got %d (adds: %d, modifies: %d)",
			totalCalls, mock.addCalls, mock.modifyCalls)
	}
}

// TestSyncContextCancel 验证 context 取消时快速失败
func TestSyncContextCancel(t *testing.T) {
	mock := newMockProvider()
	ip := net.ParseIP("2001:db8::1")

	d := &ddns.Domain{
		Domain:    "example.com",
		SubDomain: "www",
		Type:      "AAAA",
		TTL:       600,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	err := ddns.SyncRecord(ctx, d, ip, mock)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	t.Logf("got expected context error: %v", err)
}

// TestSyncRecordNameFormats 验证不同记录名格式的匹配
func TestSyncRecordNameFormats(t *testing.T) {
	tests := []struct {
		name        string
		rName       string // 记录名
		fqdn        string // 完整目标域名
		subDomain   string // 子域名标签
		shouldMatch bool
	}{
		// Cloudflare 格式：完整域名
		{name: "cloudflare full domain", rName: "www.example.com", fqdn: "www.example.com", subDomain: "www", shouldMatch: true},
		// HuaweiCloud 格式：完整域名带点号
		{name: "huaweicloud trailing dot", rName: "www.example.com.", fqdn: "www.example.com", subDomain: "www", shouldMatch: true},
		// Tencent DNSPod 格式：仅子域名标签
		{name: "tencent subdomain label", rName: "www", fqdn: "www.example.com", subDomain: "www", shouldMatch: true},
		// 根域名：@ 标签
		{name: "root domain @ label", rName: "@", fqdn: "example.com", subDomain: "@", shouldMatch: true},
		// 根域名：空字符串
		{name: "root domain empty string", rName: "", fqdn: "example.com", subDomain: "@", shouldMatch: true},
		// 不应匹配
		{name: "different subdomain", rName: "api.example.com", fqdn: "www.example.com", subDomain: "www", shouldMatch: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := false
			// 模拟 sync 中的 recordNameMatches 逻辑
			name := tt.rName
			if len(name) > 0 && name[len(name)-1] == '.' {
				name = name[:len(name)-1]
			}
			if name == tt.fqdn || name == tt.subDomain {
				matched = true
			}
			if tt.subDomain == "@" && (name == "" || name == "@") {
				matched = true
			}

			if matched != tt.shouldMatch {
				t.Errorf("match=%v, expected=%v (rName=%q, fqdn=%q, subDomain=%q)",
					matched, tt.shouldMatch, tt.rName, tt.fqdn, tt.subDomain)
			}
		})
	}
}
