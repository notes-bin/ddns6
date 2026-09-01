package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
)

// TestBuildDomains_AllSubdomains 验证多子域名均生成正确 Domain 字段。
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

// TestBuildDomains_EmptySubdomains 验证空子域名列表返回长度为 0 的非 nil 切片。
func TestBuildDomains_EmptySubdomains(t *testing.T) {
	domains := buildDomains("example.com", []string{}, 300)
	if domains == nil {
		t.Fatal("buildDomains 不应返回 nil")
	}
	if len(domains) != 0 {
		t.Errorf("期望 0 个域名, 得到 %d", len(domains))
	}
}

// TestBuildDomains_ZeroTTL 验证 TTL=0 原样写入，不做默认替换。
func TestBuildDomains_ZeroTTL(t *testing.T) {
	domains := buildDomains("example.com", []string{"www"}, 0)
	if len(domains) != 1 {
		t.Fatalf("期望 1 个域名, 得到 %d", len(domains))
	}
	if domains[0].TTL != 0 {
		t.Errorf("TTL 应为 0（传入值）, 得到 %d", domains[0].TTL)
	}
}

// TestBuildDomains_TypeIsAlwaysAAAA 验证 Domain.Type 固定为 AAAA。
func TestBuildDomains_TypeIsAlwaysAAAA(t *testing.T) {
	domains := buildDomains("example.com", []string{"www"}, 600)
	if len(domains) != 1 {
		t.Fatalf("期望 1 个域名, 得到 %d", len(domains))
	}
	if domains[0].Type != "AAAA" {
		t.Errorf("Type = %q, 期望 AAAA", domains[0].Type)
	}
}

// TestFormatProviderFlags_Empty 验证空 flag 列表格式化为空串。
func TestFormatProviderFlags_Empty(t *testing.T) {
	result := formatProviderFlags([]providerFlag{})
	if result != "" {
		t.Errorf("空 flag 列表应返回空字符串, 得到 %q", result)
	}
}

// TestFormatProviderFlags_Single 验证单 flag 的名称与 usage 出现在输出中。
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

// TestFormatProviderFlags_Multiple 验证多 flag 均出现在格式化文本中。
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

// TestFormatSampleFlags_Empty 验证空列表的示例片段为空串。
func TestFormatSampleFlags_Empty(t *testing.T) {
	result := formatSampleFlags([]providerFlag{})
	if result != "" {
		t.Errorf("空 flag 列表应返回空字符串, 得到 %q", result)
	}
}

// TestFormatSampleFlags_Single 验证单 flag 示例为 " --name YOUR_name" 形式。
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

// TestFormatSampleFlags_Multiple 验证多 flag 示例均包含对应 --name。
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

// TestRequireFlags_AllPresent 验证全部必填 flag 已设置时不报错。
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

// TestRequireFlags_MissingRequired 验证缺失必填 flag 时错误信息包含 flag 名。
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

// TestRequireFlags_EmptyFlags 验证空校验列表恒成功。
func TestRequireFlags_EmptyFlags(t *testing.T) {
	cmd := &cobra.Command{}
	err := requireFlags(cmd, []providerFlag{})
	if err != nil {
		t.Errorf("空 flag 列表不应返回错误: %v", err)
	}
}

// TestCreateProviderFromConfig_Unsupported 验证未知 provider 返回 unsupported 错误。
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

// TestCreateProviderFromConfig_EmptyProvider 验证空 provider 名视为不支持。
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

// TestGetString_NotRegistered 验证未注册 flag 时 getString 返回空串。
func TestGetString_NotRegistered(t *testing.T) {
	cmd := &cobra.Command{}
	result := getString(cmd, "non-existent")
	if result != "" {
		t.Errorf("未注册的 flag 应返回空字符串, 得到 %q", result)
	}
}

// TestGetString_RegisteredNotSet 验证已注册未赋值时返回空串。
func TestGetString_RegisteredNotSet(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	result := getString(cmd, "token")
	if result != "" {
		t.Errorf("已注册但未设置的 flag 应返回空字符串, 得到 %q", result)
	}
}

// TestGetString_Set 验证已设置字符串 flag 可正确读出。
func TestGetString_Set(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	cmd.Flags().Set("token", "abc123")
	result := getString(cmd, "token")
	if result != "abc123" {
		t.Errorf("getString = %q, 期望 %q", result, "abc123")
	}
}

// TestGetDuration_NotRegistered 验证未注册 duration 时回退为 5 分钟。
func TestGetDuration_NotRegistered(t *testing.T) {
	cmd := &cobra.Command{}
	result := getDuration(cmd, "interval")
	if result != 5*time.Minute {
		t.Errorf("未注册时默认应为 5m, 得到 %v", result)
	}
}

// TestGetDuration_Set 验证已设置 duration 可正确解析。
func TestGetDuration_Set(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Duration("interval", 5*time.Minute, "")
	cmd.Flags().Set("interval", "10m")
	result := getDuration(cmd, "interval")
	if result != 10*time.Minute {
		t.Errorf("getDuration = %v, 期望 10m", result)
	}
}

// TestCreateDomainConfigs_NoDomain 验证缺少 --domain 时报错。
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

// TestCreateDomainConfigs_Success 验证完整 flag 可构造期望 Domain。
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

// TestRestrictedProviders_Contains 验证 duckdns/he/noip 均在受限表中。
func TestRestrictedProviders_Contains(t *testing.T) {
	expected := map[string]bool{"duckdns": true, "he": true, "noip": true}
	for name := range expected {
		if !restrictedProviders[name] {
			t.Errorf("restrictedProviders 应包含 %q", name)
		}
	}
}

// TestNotRestricted 验证常见完整 API 运营商不在受限表中。
func TestNotRestricted(t *testing.T) {
	notRestricted := []string{"tencent", "cloudflare", "alicloud", "godaddy"}
	for _, name := range notRestricted {
		if restrictedProviders[name] {
			t.Errorf("%q 不应在 restrictedProviders 中", name)
		}
	}
}
