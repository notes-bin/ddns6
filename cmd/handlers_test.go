package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// mockDNS 实现 ddns.DNSProvider，供 list/clean/check 单测。
type mockDNS struct {
	records   []ddns.RecordInfo
	getErr    error
	deleteErr error
	deleted   []ddns.RecordInfo
	mu        sync.Mutex
}

func (m *mockDNS) GetRecords(context.Context, string, string) ([]ddns.RecordInfo, error) {
	return m.records, m.getErr
}

func (m *mockDNS) AddRecord(context.Context, ddns.RecordInfo) error { return nil }

func (m *mockDNS) ModifyRecord(context.Context, ddns.RecordInfo) error { return nil }

func (m *mockDNS) DeleteRecord(_ context.Context, r ddns.RecordInfo) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	m.deleted = append(m.deleted, r)
	m.mu.Unlock()
	return nil
}

func (m *mockDNS) deletedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.deleted)
}

// listCleanFlags 为 handleList/handleClean 注册所需 flag。
func listCleanFlags(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("type", "AAAA", "")
	cmd.Flags().StringArray("subdomain", []string{"@"}, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("yes", false, "")
	return cmd
}

// withArgs 临时替换 os.Args，并在测试结束时恢复。
func withArgs(t *testing.T, args ...string) {
	t.Helper()
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = args
}

// stubServiceRunner 替换 serviceRunner，测试结束时恢复。
func stubServiceRunner(t *testing.T, fn func([]*ddns.Domain, ddns.DNSProvider, time.Duration, []ipaddr.IPv6Fetcher, string) error) {
	t.Helper()
	orig := serviceRunner
	t.Cleanup(func() { serviceRunner = orig })
	if fn == nil {
		fn = func([]*ddns.Domain, ddns.DNSProvider, time.Duration, []ipaddr.IPv6Fetcher, string) error {
			return nil
		}
	}
	serviceRunner = fn
}

// writeTestConfig 在临时 HOME 下写入最小可用配置。
func writeTestConfig(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".ddns6")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// withStdinLine 将一行文本注入 os.Stdin，测试结束时恢复。
func withStdinLine(t *testing.T, line string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() {
		fmt.Fprintln(w, line)
		w.Close()
	}()
}

// requireContains 断言 got 包含所有 want 子串。
func requireContains(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("期望包含 %q, got %q", want, got)
		}
	}
}

// requireErrContains 断言 err 非 nil 且错误信息包含 substr。
func requireErrContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Fatalf("期望错误含 %q, got %v", substr, err)
	}
}

// captureStdout 捕获 fn 执行期间的标准输出。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestRecordTypeDesc 验证空类型与具体类型的标题文案。
func TestRecordTypeDesc(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "DNS records"},
		{"AAAA", "AAAA records"},
		{"A", "A records"},
	}
	for _, tt := range tests {
		if got := recordTypeDesc(tt.in); got != tt.want {
			t.Errorf("recordTypeDesc(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestBuildFilterInfo 验证去重后的 FQDN 拼接。
func TestBuildFilterInfo(t *testing.T) {
	domains := []*ddns.Domain{
		{Domain: "example.com", SubDomain: "www"},
		{Domain: "example.com", SubDomain: "www"},
		{Domain: "example.com", SubDomain: "@"},
	}
	got := buildFilterInfo(domains)
	requireContains(t, got, "www.example.com", "example.com")
	if strings.Count(got, "www.example.com") != 1 {
		t.Errorf("重复 FQDN 应去重, got %q", got)
	}
}

// TestHandleList_EmptyAndFound 覆盖无记录与有记录输出。
func TestHandleList_EmptyAndFound(t *testing.T) {
	domains := []*ddns.Domain{{Domain: "example.com", SubDomain: "www", Type: "AAAA"}}

	t.Run("empty", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := handleList(listCleanFlags(t), domains, &mockDNS{}); err != nil {
				t.Fatalf("handleList: %v", err)
			}
		})
		requireContains(t, out, "No records found")
	})

	t.Run("found", func(t *testing.T) {
		cmd := listCleanFlags(t)
		cmd.Flags().Set("subdomain", "www")
		cmd.Flags().Lookup("subdomain").Changed = true
		m := &mockDNS{records: []ddns.RecordInfo{
			{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1"},
		}}
		out := captureStdout(t, func() {
			if err := handleList(cmd, domains, m); err != nil {
				t.Fatalf("handleList: %v", err)
			}
		})
		requireContains(t, out, "Found 1", "filtered by subdomain")
	})

	t.Run("query error", func(t *testing.T) {
		if err := handleList(listCleanFlags(t), domains, &mockDNS{getErr: errors.New("boom")}); err == nil {
			t.Fatal("查询失败应返回错误")
		}
	})
}

// TestHandleClean 覆盖 dry-run、确认、--yes 删除与失败路径。
func TestHandleClean(t *testing.T) {
	domains := []*ddns.Domain{{Domain: "example.com", SubDomain: "www", Type: "AAAA"}}
	rec := ddns.RecordInfo{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1"}

	t.Run("no records", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := handleClean(listCleanFlags(t), domains, &mockDNS{}); err != nil {
				t.Fatalf("handleClean: %v", err)
			}
		})
		requireContains(t, out, "No matching records")
	})

	t.Run("dry-run", func(t *testing.T) {
		cmd := listCleanFlags(t)
		cmd.Flags().Set("dry-run", "true")
		m := &mockDNS{records: []ddns.RecordInfo{rec}}
		out := captureStdout(t, func() {
			if err := handleClean(cmd, domains, m); err != nil {
				t.Fatalf("handleClean: %v", err)
			}
		})
		requireContains(t, out, "Dry-run")
		if m.deletedCount() != 0 {
			t.Error("dry-run 不应实际删除")
		}
	})

	t.Run("yes delete", func(t *testing.T) {
		cmd := listCleanFlags(t)
		cmd.Flags().Set("yes", "true")
		m := &mockDNS{records: []ddns.RecordInfo{rec}}
		out := captureStdout(t, func() {
			if err := handleClean(cmd, domains, m); err != nil {
				t.Fatalf("handleClean: %v", err)
			}
		})
		if m.deletedCount() != 1 {
			t.Fatalf("应删除 1 条, got %d", m.deletedCount())
		}
		requireContains(t, out, "Deleted 1")
	})

	t.Run("confirm yes", func(t *testing.T) {
		withStdinLine(t, "yes")
		m := &mockDNS{records: []ddns.RecordInfo{rec}}
		out := captureStdout(t, func() {
			if err := handleClean(listCleanFlags(t), domains, m); err != nil {
				t.Fatalf("handleClean: %v", err)
			}
		})
		if m.deletedCount() != 1 {
			t.Fatalf("应删除 1 条, got %d, out=%q", m.deletedCount(), out)
		}
	})

	t.Run("confirm cancel", func(t *testing.T) {
		withStdinLine(t, "n")
		m := &mockDNS{records: []ddns.RecordInfo{rec}}
		out := captureStdout(t, func() {
			if err := handleClean(listCleanFlags(t), domains, m); err != nil {
				t.Fatalf("handleClean: %v", err)
			}
		})
		requireContains(t, out, "Cancelled")
		if m.deletedCount() != 0 {
			t.Error("取消后不应删除")
		}
	})

	t.Run("delete failed", func(t *testing.T) {
		cmd := listCleanFlags(t)
		cmd.Flags().Set("yes", "true")
		m := &mockDNS{records: []ddns.RecordInfo{rec}, deleteErr: errors.New("deny")}
		if err := handleClean(cmd, domains, m); err == nil {
			t.Fatal("删除失败应返回错误")
		}
	})

	t.Run("query error", func(t *testing.T) {
		if err := handleClean(listCleanFlags(t), domains, &mockDNS{getErr: errors.New("boom")}); err == nil {
			t.Fatal("查询失败应返回错误")
		}
	})
}

// TestCheckFromConfig 覆盖配置校验各早退与未知 provider。
func TestCheckFromConfig(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *config.Config
		substr string
	}{
		{name: "empty provider", cfg: &config.Config{}, substr: "provider: empty"},
		{name: "empty domain", cfg: &config.Config{Provider: "tencent"}, substr: "domain: empty"},
		{name: "empty auth", cfg: &config.Config{Provider: "tencent", Domain: "example.com", Subdomains: []string{"@"}}, substr: "auth: empty"},
		{name: "unknown provider", cfg: &config.Config{
			Provider: "not-real", Domain: "example.com", Subdomains: []string{"@"},
			Auth: map[string]string{"k": "v"},
		}, substr: "Unknown provider"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := checkFromConfig(tt.cfg); err != nil {
					t.Fatalf("checkFromConfig: %v", err)
				}
			})
			requireContains(t, out, tt.substr)
		})
	}
}

// TestCheckFromConfig_APIPath 走完整校验并用假凭据触发 API（失败仍返回 nil）。
func TestCheckFromConfig_APIPath(t *testing.T) {
	cfg := &config.Config{
		Provider:   "cloudflare",
		Domain:     "example.com",
		Subdomains: []string{"www"},
		Auth:       map[string]string{"api_token": "fake-token-for-test"},
		Interval:   "10m",
		TTL:        300,
	}
	out := captureStdout(t, func() {
		if err := checkFromConfig(cfg); err != nil {
			t.Fatalf("checkFromConfig: %v", err)
		}
	})
	requireContains(t, out, "Provider 'cloudflare' is valid", "API Connectivity Test")
}

// TestCreateProviderFromConfig_Success 验证已知 provider 可从配置创建。
func TestCreateProviderFromConfig_Success(t *testing.T) {
	cfg := &config.Config{
		Provider: "cloudflare",
		Auth:     map[string]string{"api_token": "tok"},
	}
	p, err := createProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("createProviderFromConfig: %v", err)
	}
	if p == nil {
		t.Fatal("provider 不应为 nil")
	}
}

// TestCreateDomainConfigs_DefaultSubdomain 验证未指定子域名时默认为 @。
func TestCreateDomainConfigs_DefaultSubdomain(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("domain", "", "")
	cmd.Flags().StringArray("subdomain", nil, "")
	cmd.Flags().Int("ttl", 600, "")
	cmd.Flags().Set("domain", "example.com")

	domains, err := createDomainConfigs(cmd)
	if err != nil {
		t.Fatalf("createDomainConfigs: %v", err)
	}
	if len(domains) != 1 || domains[0].SubDomain != "@" {
		t.Fatalf("默认子域名应为 @, got %+v", domains)
	}
}

// TestRequireFlags_InvalidFlag 验证未注册 flag 时返回错误。
func TestRequireFlags_InvalidFlag(t *testing.T) {
	if err := requireFlags(&cobra.Command{}, []providerFlag{{name: "missing-flag"}}); err == nil {
		t.Fatal("未注册 flag 应返回错误")
	}
}

// TestRunWithConfig_LoadError 验证无配置文件时 runWithConfig 报错。
func TestRunWithConfig_LoadError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	err := runWithConfig(&cobra.Command{}, "list", func(*cobra.Command, *config.Config, []*ddns.Domain, ddns.DNSProvider) error {
		t.Fatal("配置缺失时不应调用 handler")
		return nil
	})
	if err == nil {
		t.Fatal("无配置时应返回错误")
	}
}

// TestRunWithConfig_Success 验证配置加载后调用 handler。
func TestRunWithConfig_Success(t *testing.T) {
	writeTestConfig(t, `
provider: cloudflare
domain: example.com
subdomains:
  - www
auth:
  api_token: "tok"
`)
	called := false
	err := runWithConfig(&cobra.Command{}, "list", func(_ *cobra.Command, cfg *config.Config, domains []*ddns.Domain, p ddns.DNSProvider) error {
		called = true
		if cfg.Provider != "cloudflare" {
			t.Errorf("provider = %q", cfg.Provider)
		}
		if len(domains) != 1 {
			t.Errorf("domains len = %d", len(domains))
		}
		if p == nil {
			t.Error("provider nil")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("runWithConfig: %v", err)
	}
	if !called {
		t.Fatal("应调用 handler")
	}
}

// TestRunListCleanWithConfig_Restricted 验证受限运营商拒绝 list/clean。
func TestRunListCleanWithConfig_Restricted(t *testing.T) {
	writeTestConfig(t, `
provider: duckdns
domain: example.com
subdomains:
  - "@"
auth:
  token: "tok"
`)
	cmd := listCleanFlags(t)
	requireErrContains(t, runListWithConfig(cmd), "does not support 'list'")
	requireErrContains(t, runCleanWithConfig(cmd), "does not support 'clean'")
}

// TestRunServiceFromConfigHandler 覆盖 interval/interface 覆盖与 serviceRunner 调用。
func TestRunServiceFromConfigHandler(t *testing.T) {
	var gotInterval time.Duration
	var gotIface string
	stubServiceRunner(t, func(_ []*ddns.Domain, _ ddns.DNSProvider, interval time.Duration, _ []ipaddr.IPv6Fetcher, iface string) error {
		gotInterval = interval
		gotIface = iface
		return nil
	})

	t.Run("invalid interval", func(t *testing.T) {
		err := runServiceFromConfigHandler(nil, &config.Config{Interval: "bad"}, nil, &mockDNS{})
		if err == nil {
			t.Fatal("非法 interval 应返回错误")
		}
	})

	t.Run("flag overrides", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Duration("interval", 5*time.Minute, "")
		cmd.Flags().String("interface", "", "")
		cmd.Flags().Set("interval", "2m")
		cmd.Flags().Set("interface", "en0")
		cmd.Flags().Lookup("interval").Changed = true
		cmd.Flags().Lookup("interface").Changed = true

		cfg := &config.Config{Interval: "10m", Interface: "eth0"}
		if err := runServiceFromConfigHandler(cmd, cfg, nil, &mockDNS{}); err != nil {
			t.Fatalf("runServiceFromConfigHandler: %v", err)
		}
		if gotInterval != 2*time.Minute {
			t.Errorf("interval = %v, want 2m", gotInterval)
		}
		if gotIface != "en0" {
			t.Errorf("iface = %q, want en0", gotIface)
		}
	})
}
