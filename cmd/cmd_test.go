// Package cmd 命令行工具测试
package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
)

// ============================================================
// buildDomains 测试
// ============================================================

func TestBuildDomains_AllSubdomains(t *testing.T) {
	domains := buildDomains("example.com", []string{"www", "@", "api"}, 600)
	if len(domains) != 3 {
		t.Fatalf("期望 3 个域名, 得到 %d", len(domains))
	}

	checks := []struct {
		domain, subDomain string
		ttl               int
	}{
		{"example.com", "www", 600},
		{"example.com", "@", 600},
		{"example.com", "api", 600},
	}
	for i, c := range checks {
		if domains[i].Domain != c.domain {
			t.Errorf("domains[%d].Domain = %q, 期望 %q", i, domains[i].Domain, c.domain)
		}
		if domains[i].SubDomain != c.subDomain {
			t.Errorf("domains[%d].SubDomain = %q, 期望 %q", i, domains[i].SubDomain, c.subDomain)
		}
		if domains[i].TTL != c.ttl {
			t.Errorf("domains[%d].TTL = %d, 期望 %d", i, domains[i].TTL, c.ttl)
		}
		if domains[i].Type != "AAAA" {
			t.Errorf("domains[%d].Type = %q, 期望 AAAA", i, domains[i].Type)
		}
	}
}

func TestBuildDomains_EmptySubdomains(t *testing.T) {
	domains := buildDomains("example.com", []string{}, 300)
	if domains == nil {
		t.Fatal("buildDomains 不应返回 nil")
	}
	if len(domains) != 0 {
		t.Errorf("期望 0 个域名, 得到 %d", len(domains))
	}
}

func TestBuildDomains_ZeroTTL(t *testing.T) {
	domains := buildDomains("example.com", []string{"www"}, 0)
	if len(domains) != 1 {
		t.Fatalf("期望 1 个域名, 得到 %d", len(domains))
	}
	if domains[0].TTL != 0 {
		t.Errorf("TTL 应为 0（传入值）, 得到 %d", domains[0].TTL)
	}
}

func TestBuildDomains_TypeIsAlwaysAAAA(t *testing.T) {
	domains := buildDomains("example.com", []string{"www"}, 600)
	if len(domains) != 1 {
		t.Fatalf("期望 1 个域名, 得到 %d", len(domains))
	}
	if domains[0].Type != "AAAA" {
		t.Errorf("Type = %q, 期望 AAAA", domains[0].Type)
	}
}

// ============================================================
// formatProviderFlags 测试
// ============================================================

func TestFormatProviderFlags_Empty(t *testing.T) {
	result := formatProviderFlags([]providerFlag{})
	if result != "" {
		t.Errorf("空 flag 列表应返回空字符串, 得到 %q", result)
	}
}

func TestFormatProviderFlags_Single(t *testing.T) {
	flags := []providerFlag{
		{name: "api-token", usage: "Cloudflare API Token"},
	}
	result := formatProviderFlags(flags)
	if !strings.Contains(result, "api-token") {
		t.Error("结果应包含 flag 名")
	}
	if !strings.Contains(result, "Cloudflare") {
		t.Error("结果应包含 usage 说明")
	}
}

func TestFormatProviderFlags_Multiple(t *testing.T) {
	flags := []providerFlag{
		{name: "secret-id", usage: "Secret ID"},
		{name: "secret-key", usage: "Secret Key"},
	}
	result := formatProviderFlags(flags)
	if !strings.Contains(result, "secret-id") {
		t.Error("结果应包含 secret-id")
	}
	if !strings.Contains(result, "secret-key") {
		t.Error("结果应包含 secret-key")
	}
}

// ============================================================
// formatSampleFlags 测试
// ============================================================

func TestFormatSampleFlags_Empty(t *testing.T) {
	result := formatSampleFlags([]providerFlag{})
	if result != "" {
		t.Errorf("空 flag 列表应返回空字符串, 得到 %q", result)
	}
}

func TestFormatSampleFlags_Single(t *testing.T) {
	flags := []providerFlag{
		{name: "api-token"},
	}
	result := formatSampleFlags(flags)
	expected := " --api-token YOUR_api-token"
	if result != expected {
		t.Errorf("= %q, 期望 %q", result, expected)
	}
}

func TestFormatSampleFlags_Multiple(t *testing.T) {
	flags := []providerFlag{
		{name: "secret-id"},
		{name: "secret-key"},
	}
	result := formatSampleFlags(flags)
	if !strings.Contains(result, "--secret-id") {
		t.Error("结果应包含 --secret-id")
	}
	if !strings.Contains(result, "--secret-key") {
		t.Error("结果应包含 --secret-key")
	}
}

// ============================================================
// requireFlags 测试
// ============================================================

func TestRequireFlags_AllPresent(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("api-token", "", "")
	cmd.Flags().String("domain", "", "")
	cmd.Flags().Set("api-token", "abc123")
	cmd.Flags().Set("domain", "example.com")

	cmd.Flags().Lookup("api-token").Changed = true
	cmd.Flags().Lookup("domain").Changed = true

	err := requireFlags(cmd, []providerFlag{
		{name: "api-token"},
		{name: "domain"},
	})
	if err != nil {
		t.Errorf("所有 flag 都存在时不应返回错误: %v", err)
	}
}

func TestRequireFlags_MissingRequired(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("api-token", "", "")

	err := requireFlags(cmd, []providerFlag{
		{name: "api-token"},
	})
	if err == nil {
		t.Fatal("缺少必填 flag 时应返回错误")
	}
	if !strings.Contains(err.Error(), "api-token") {
		t.Errorf("错误信息应包含缺失的 flag 名, 得到: %v", err)
	}
}

func TestRequireFlags_EmptyFlags(t *testing.T) {
	cmd := &cobra.Command{}
	err := requireFlags(cmd, []providerFlag{})
	if err != nil {
		t.Errorf("空 flag 列表不应返回错误: %v", err)
	}
}

// ============================================================
// createProviderFromConfig 测试
// ============================================================

func TestCreateProviderFromConfig_Unsupported(t *testing.T) {
	cfg := &config.Config{
		Provider: "invalid_provider",
		Domain:   "example.com",
	}
	_, err := createProviderFromConfig(cfg)
	if err == nil {
		t.Fatal("不支持的 provider 应返回错误")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("错误信息应提示不支持, 得到: %v", err)
	}
}

func TestCreateProviderFromConfig_EmptyProvider(t *testing.T) {
	cfg := &config.Config{
		Provider: "",
		Domain:   "example.com",
	}
	_, err := createProviderFromConfig(cfg)
	if err == nil {
		t.Fatal("空 provider 应返回错误")
	}
}

// ============================================================
// getString / getDuration 测试
// ============================================================

func TestGetString_NotRegistered(t *testing.T) {
	cmd := &cobra.Command{}
	result := getString(cmd, "non-existent")
	if result != "" {
		t.Errorf("未注册的 flag 应返回空字符串, 得到 %q", result)
	}
}

func TestGetString_RegisteredNotSet(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	result := getString(cmd, "token")
	if result != "" {
		t.Errorf("已注册但未设置的 flag 应返回空字符串, 得到 %q", result)
	}
}

func TestGetString_Set(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	cmd.Flags().Set("token", "abc123")
	result := getString(cmd, "token")
	if result != "abc123" {
		t.Errorf("getString = %q, 期望 %q", result, "abc123")
	}
}

func TestGetDuration_NotRegistered(t *testing.T) {
	cmd := &cobra.Command{}
	result := getDuration(cmd, "interval")
	if result != 5*time.Minute {
		t.Errorf("未注册时默认应为 5m, 得到 %v", result)
	}
}

func TestGetDuration_Set(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Duration("interval", 5*time.Minute, "")
	cmd.Flags().Set("interval", "10m")
	result := getDuration(cmd, "interval")
	if result != 10*time.Minute {
		t.Errorf("getDuration = %v, 期望 10m", result)
	}
}

// ============================================================
// createDomainConfigs 测试
// ============================================================

func TestCreateDomainConfigs_NoDomain(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("domain", "", "")
	cmd.Flags().StringArray("subdomain", []string{"@"}, "")
	cmd.Flags().Int("ttl", 600, "")

	_, err := createDomainConfigs(cmd)
	if err == nil {
		t.Fatal("缺少 --domain 时应返回错误")
	}
	if !strings.Contains(err.Error(), "--domain") {
		t.Errorf("错误信息应提示 --domain, 得到: %v", err)
	}
}

func TestCreateDomainConfigs_Success(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("domain", "", "")
	cmd.Flags().StringArray("subdomain", []string{"@"}, "")
	cmd.Flags().Int("ttl", 600, "")
	cmd.Flags().Set("domain", "example.com")
	cmd.Flags().Set("subdomain", "www")

	domains, err := createDomainConfigs(cmd)
	if err != nil {
		t.Fatalf("createDomainConfigs 不应返回错误: %v", err)
	}
	if len(domains) == 0 {
		t.Fatal("应返回至少一个域名")
	}
	if domains[0].Domain != "example.com" {
		t.Errorf("Domain = %q, 期望 %q", domains[0].Domain, "example.com")
	}
	if domains[0].SubDomain != "www" {
		t.Errorf("SubDomain = %q, 期望 %q", domains[0].SubDomain, "www")
	}
}

// ============================================================
// restrictedProviders 测试
// ============================================================

func TestRestrictedProviders_Contains(t *testing.T) {
	expected := map[string]bool{"duckdns": true, "he": true, "noip": true}
	for name := range expected {
		if !restrictedProviders[name] {
			t.Errorf("restrictedProviders 应包含 %q", name)
		}
	}
}

func TestNotRestricted(t *testing.T) {
	notRestricted := []string{"tencent", "cloudflare", "alicloud", "godaddy"}
	for _, name := range notRestricted {
		if restrictedProviders[name] {
			t.Errorf("%q 不应在 restrictedProviders 中", name)
		}
	}
}
