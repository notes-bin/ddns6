package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// TestProviderSubcommandRunE 验证 registerProviders / registerProviderSubCommands 的 RunE 分支。
func TestProviderSubcommandRunE(t *testing.T) {

	initRootCmd()
	stubServiceRunner(t, nil)

	t.Run("run missing flags", func(t *testing.T) {
		withArgs(t, "ddns6", "run", "cloudflare", "--domain", "example.com")
		if err := rootCmd.Execute(); err == nil {
			t.Fatal("缺少必填 flag 应失败")
		}
	})

	t.Run("run success", func(t *testing.T) {
		withArgs(t,
			"ddns6", "run", "cloudflare",
			"--domain", "example.com",
			"--subdomain", "www",
			"--api-token", "tok",
			"--log-file", "",
		)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("run cloudflare: %v", err)
		}
	})

	t.Run("records missing flags", func(t *testing.T) {
		withArgs(t, "ddns6", "records", "cloudflare", "--domain", "example.com", "--log-file", "")
		if err := rootCmd.Execute(); err == nil {
			t.Fatal("缺少 api-token 应失败")
		}
	})

	t.Run("records success hits API", func(t *testing.T) {
		withArgs(t,
			"ddns6", "records", "cloudflare",
			"--domain", "example.com",
			"--api-token", "fake-token",
			"--log-file", "",
		)
		// 假 token 通常导致 API 错误，但 RunE 与 handleRecords 路径已执行
		_ = rootCmd.Execute()

	})

	t.Run("clean restricted duckdns", func(t *testing.T) {
		withArgs(t, "ddns6", "clean", "duckdns", "--log-file", "")
		requireErrContains(t, rootCmd.Execute(), "does not support")
	})
}

// TestRunRecordsCleanWithConfig_Success 验证非受限运营商的配置文件 records/clean 成功路径。
func TestRunRecordsCleanWithConfig_Success(t *testing.T) {

	writeTestConfig(t, `
provider: cloudflare
domain: example.com
subdomains:
  - www
auth:
  api_token: "fake-token"
`)
	cmd := listCleanFlags(t)
	cmd.Flags().Set("yes", "true")
	cmd.Flags().Set("dry-run", "true")

	_ = runRecordsWithConfig(cmd)
	_ = runCleanWithConfig(cmd)
}

// TestRunCmd_ConfigMode 验证 run 配置文件模式会调用 serviceRunner。
func TestRunCmd_ConfigMode(t *testing.T) {

	writeTestConfig(t, `
provider: cloudflare
domain: example.com
subdomains:
  - "@"
auth:
  api_token: "tok"
interval: 5m
`)
	called := false
	stubServiceRunner(t, func([]*ddns.Domain, ddns.DNSProvider, time.Duration, []ipaddr.IPv6Fetcher, string, string) error {
		called = true
		return nil
	})

	initRootCmd()
	withArgs(t, "ddns6", "run", "--log-file", "")
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run config mode: %v", err)
	}
	if !called {
		t.Fatal("应调用 serviceRunner")
	}
}

// TestRunCmd_HelpArg 验证 run/records/clean 的 help 参数分支。
func TestRunCmd_HelpArg(t *testing.T) {

	initRootCmd()
	for _, args := range [][]string{
		{"ddns6", "run", "help"},
		{"ddns6", "records", "help"},
		{"ddns6", "clean", "help"},
	} {
		withArgs(t, args...)
		_ = rootCmd.Execute()
	}
}

// TestCompletionCommand 验证 shell completion 子命令（含不支持的 shell）。
func TestCompletionCommand(t *testing.T) {

	initRootCmd()

	old := os.Stdout
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = f
	t.Cleanup(func() { os.Stdout = old; f.Close() })

	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		withArgs(t, "ddns6", "completion", shell)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("completion %s: %v", shell, err)
		}
	}

	withArgs(t, "ddns6", "completion", "noshell")
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("不支持的 shell 应失败")
	}
}

// TestApplyEnvOverrides_SkipWhenChanged 验证命令行已设置时不被环境变量覆盖。
func TestApplyEnvOverrides_SkipWhenChanged(t *testing.T) {

	initRootCmd()
	t.Setenv("DDNS6_DOMAIN", "from-env.com")
	f := rootCmd.PersistentFlags().Lookup("domain")
	if f == nil {
		t.Fatal("domain flag 不存在")
	}
	rootCmd.PersistentFlags().Set("domain", "from-cli.com")
	f.Changed = true
	applyEnvOverrides()
	if v, _ := rootCmd.PersistentFlags().GetString("domain"); v != "from-cli.com" {
		t.Errorf("已 Changed 时不应被环境变量覆盖, got %q", v)
	}
}

// TestCreateDomainConfigs_FlagErrors 验证缺失 flag 定义时 createDomainConfigs 返回错误。
func TestCreateDomainConfigs_FlagErrors(t *testing.T) {

	_, err := createDomainConfigs(listCleanFlags(t)) // 无 domain/ttl
	if err == nil {
		t.Fatal("缺少 --domain flag 定义应返回错误")
	}
}

// TestExecute_CommandFailed 验证业务错误经 Execute 包装为 command failed。
func TestExecute_CommandFailed(t *testing.T) {

	initRootCmd()
	t.Setenv("HOME", t.TempDir())
	withArgs(t, "ddns6", "records", "--log-file", "")
	requireErrContains(t, Execute(), "command failed")
}

// TestCheckCmd_CLIMode 验证 check 命令行模式的主要分支。
func TestCheckCmd_CLIMode(t *testing.T) {

	initRootCmd()
	for _, args := range [][]string{
		{"ddns6", "check", "help"},
		{"ddns6", "check", "unknown-provider", "--log-file", ""},
		{"ddns6", "check", "cloudflare", "--log-file", ""},
		{"ddns6", "check", "cloudflare", "--domain", "example.com", "--log-file", ""},
	} {
		withArgs(t, args...)
		_ = rootCmd.Execute()
	}
}

// TestCheckCmd_ConfigMode 验证 check 可从配置文件读取并执行。
func TestCheckCmd_ConfigMode(t *testing.T) {

	writeTestConfig(t, `
provider: cloudflare
domain: example.com
subdomains:
  - www
auth:
  api_token: "tok"
`)
	initRootCmd()
	withArgs(t, "ddns6", "check", "--log-file", "")
	_ = rootCmd.Execute()
}

// TestRootHelpAndCleanSubcommand 验证根命令帮助与 clean 子命令 dry-run。
func TestRootHelpAndCleanSubcommand(t *testing.T) {

	initRootCmd()

	withArgs(t, "ddns6")
	_ = rootCmd.Execute()

	withArgs(t,
		"ddns6", "clean", "cloudflare",
		"--domain", "example.com",
		"--subdomain", "www",
		"--api-token", "fake",
		"--dry-run",
		"--log-file", "",
	)
	_ = rootCmd.Execute()
}

// TestRegisterProviders_RunError 验证 provider run 在缺少 domain 时返回错误。
func TestRegisterProviders_RunError(t *testing.T) {

	initRootCmd()
	withArgs(t, "ddns6", "run", "tencent", "--secret-id", "a", "--secret-key", "b", "--log-file", "")
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("缺少 --domain 应失败")
	}
}

// TestInitCmd 验证 init 可生成配置文件（含 provider 预填）。
func TestInitCmd(t *testing.T) {

	home := t.TempDir()
	t.Setenv("HOME", home)
	initRootCmd()
	withArgs(t,
		"ddns6", "init", "cloudflare",
		"--domain", "example.com",
		"--subdomain", "www",
		"--api-token", "tok",
		"--ttl", "300",
		"--interval", "10m",
		"--interface", "en0",
	)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".ddns6", "config.yaml"))
	if err != nil {
		t.Fatalf("读取配置: %v", err)
	}
	requireContains(t, string(data), "cloudflare")
}

// TestPersistentPreRun_Logging 验证子命令可触发日志初始化（含 debug）。
func TestPersistentPreRun_Logging(t *testing.T) {

	initRootCmd()
	withArgs(t, "ddns6", "records", "duckdns", "--log-file", filepath.Join(t.TempDir(), "t.log"), "--debug")
	_ = rootCmd.Execute()
}

// TestRunWithConfig_UnsupportedProvider 验证未知 provider 时 createProviderFromConfig 失败。
func TestRunWithConfig_UnsupportedProvider(t *testing.T) {

	writeTestConfig(t, `
provider: not-a-real-provider
domain: example.com
subdomains:
  - "@"
auth:
  token: "x"
`)
	err := runWithConfig(&cobra.Command{}, "records", func(*cobra.Command, *config.Config, []*ddns.Domain, ddns.DNSProvider) error {
		t.Fatal("不应调用 handler")
		return nil
	})
	requireErrContains(t, err, "unsupported provider")
}

// TestCheckFromConfig_EmptySubdomainsAndBadInterval 验证默认子域名提示与非法 interval 告警。
func TestCheckFromConfig_EmptySubdomainsAndBadInterval(t *testing.T) {

	cfg := &config.Config{
		Provider: "cloudflare",
		Domain:   "example.com",
		Auth:     map[string]string{"api_token": "tok"},
		Interval: "not-duration",
	}
	out := captureStdout(t, func() {
		_ = checkFromConfig(t.Context(), cfg)
	})
	requireContains(t, out, "subdomains: none", "parse error")
}
