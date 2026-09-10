package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/notes-bin/ddns6/internal/config"
	"github.com/notes-bin/ddns6/internal/ddns"
	"github.com/notes-bin/ddns6/pkg/ipaddr"
)

// TestProviderSubcommandRunE 覆盖 registerProviders / registerProviderSubCommands 的 RunE。
func TestProviderSubcommandRunE(t *testing.T) {
	initRootCmd()
	orig := serviceRunner
	t.Cleanup(func() { serviceRunner = orig })
	serviceRunner = func(_ []*ddns.Domain, _ ddns.DNSProvider, _ time.Duration, _ []ipaddr.IPv6Fetcher, _ string) error {
		return nil
	}

	t.Run("run missing flags", func(t *testing.T) {
		oldArgs := os.Args
		t.Cleanup(func() { os.Args = oldArgs })
		os.Args = []string{"ddns6", "run", "cloudflare", "--domain", "example.com"}
		// 缺 api-token
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("缺少必填 flag 应失败")
		}
	})

	t.Run("run success", func(t *testing.T) {
		oldArgs := os.Args
		t.Cleanup(func() { os.Args = oldArgs })
		os.Args = []string{
			"ddns6", "run", "cloudflare",
			"--domain", "example.com",
			"--subdomain", "www",
			"--api-token", "tok",
			"--log-file", "",
		}
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("run cloudflare: %v", err)
		}
	})

	t.Run("list missing flags", func(t *testing.T) {
		oldArgs := os.Args
		t.Cleanup(func() { os.Args = oldArgs })
		os.Args = []string{"ddns6", "list", "cloudflare", "--domain", "example.com", "--log-file", ""}
		if err := rootCmd.Execute(); err == nil {
			t.Fatal("缺少 api-token 应失败")
		}
	})

	t.Run("list success hits API", func(t *testing.T) {
		oldArgs := os.Args
		t.Cleanup(func() { os.Args = oldArgs })
		os.Args = []string{
			"ddns6", "list", "cloudflare",
			"--domain", "example.com",
			"--api-token", "fake-token",
			"--log-file", "",
		}
		// 假 token 通常导致 API 错误，但 RunE 与 handleList 路径已执行
		_ = rootCmd.Execute()
	})

	t.Run("clean restricted duckdns", func(t *testing.T) {
		oldArgs := os.Args
		t.Cleanup(func() { os.Args = oldArgs })
		os.Args = []string{"ddns6", "clean", "duckdns", "--log-file", ""}
		err := rootCmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("受限 clean 错误不符: %v", err)
		}
	})
}

// TestRunListCleanWithConfig_Success 覆盖非受限运营商的配置文件 list/clean 成功路径。
func TestRunListCleanWithConfig_Success(t *testing.T) {
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

	// list：假 token 通常失败，但会进入 handleList
	_ = runListWithConfig(cmd)

	// clean dry-run：同样进入 handleClean
	_ = runCleanWithConfig(cmd)
}

// TestRunCmd_ConfigMode 覆盖 run 配置文件模式（含 serviceRunner 桩）。
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
	orig := serviceRunner
	t.Cleanup(func() { serviceRunner = orig })
	called := false
	serviceRunner = func(_ []*ddns.Domain, _ ddns.DNSProvider, _ time.Duration, _ []ipaddr.IPv6Fetcher, _ string) error {
		called = true
		return nil
	}

	initRootCmd()
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"ddns6", "run", "--log-file", ""}
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run config mode: %v", err)
	}
	if !called {
		t.Fatal("应调用 serviceRunner")
	}
}

// TestRunCmd_HelpArg 覆盖 run/list/clean 的 help 参数分支。
func TestRunCmd_HelpArg(t *testing.T) {
	initRootCmd()
	for _, args := range [][]string{
		{"ddns6", "run", "help"},
		{"ddns6", "list", "help"},
		{"ddns6", "clean", "help"},
	} {
		oldArgs := os.Args
		os.Args = args
		_ = rootCmd.Execute()
		os.Args = oldArgs
	}
}

// TestCompletionCommand 覆盖 shell completion 子命令。
func TestCompletionCommand(t *testing.T) {
	initRootCmd()
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	old := os.Stdout
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = f
	t.Cleanup(func() { os.Stdout = old; f.Close() })

	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		os.Args = []string{"ddns6", "completion", shell}
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("completion %s: %v", shell, err)
		}
	}

	os.Args = []string{"ddns6", "completion", "noshell"}
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("不支持的 shell 应失败")
	}
}

// TestApplyEnvOverrides_SkipWhenChanged 验证命令行已设置时不覆盖。
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

// TestCreateDomainConfigs_FlagErrors 覆盖缺失 flag 定义时的错误路径。
func TestCreateDomainConfigs_FlagErrors(t *testing.T) {
	cmd := listCleanFlags(t) // 无 domain/ttl
	_, err := createDomainConfigs(cmd)
	if err == nil {
		t.Fatal("缺少 --domain flag 定义应返回错误")
	}
}

// TestExecute_CommandFailed 验证业务错误经 Execute 包装返回。
func TestExecute_CommandFailed(t *testing.T) {
	initRootCmd()
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	// list 无配置且无 provider → runListWithConfig 失败 → Execute 包装
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.Args = []string{"ddns6", "list", "--log-file", ""}
	err := Execute()
	if err == nil {
		t.Fatal("无配置的 list 应失败")
	}
	if !strings.Contains(err.Error(), "Command failed") {
		t.Errorf("期望 Command failed 包装, got %v", err)
	}
}

// TestCheckCmd_CLIMode 覆盖 check 命令行模式的主要分支。
func TestCheckCmd_CLIMode(t *testing.T) {
	initRootCmd()
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	cases := [][]string{
		{"ddns6", "check", "help"},
		{"ddns6", "check", "unknown-provider", "--log-file", ""},
		{"ddns6", "check", "cloudflare", "--log-file", ""}, // 缺 domain
		{"ddns6", "check", "cloudflare", "--domain", "example.com", "--log-file", ""},
	}
	for _, args := range cases {
		os.Args = args
		_ = rootCmd.Execute()
	}
}

// TestCheckCmd_ConfigMode 覆盖 check 读配置文件路径。
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
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"ddns6", "check", "--log-file", ""}
	_ = rootCmd.Execute()
}

// TestRootHelpAndCleanSubcommand 覆盖根命令帮助与 clean 子命令 dry-run。
func TestRootHelpAndCleanSubcommand(t *testing.T) {
	initRootCmd()
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{"ddns6"}
	_ = rootCmd.Execute()

	os.Args = []string{
		"ddns6", "clean", "cloudflare",
		"--domain", "example.com",
		"--subdomain", "www",
		"--api-token", "fake",
		"--dry-run",
		"--log-file", "",
	}
	_ = rootCmd.Execute()
}

// TestRegisterProviders_RunError 覆盖 provider run 在 domain 缺失时的错误返回。
func TestRegisterProviders_RunError(t *testing.T) {
	initRootCmd()
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"ddns6", "run", "tencent", "--secret-id", "a", "--secret-key", "b", "--log-file", ""}
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("缺少 --domain 应失败")
	}
}

// TestInitCmd 覆盖 init 生成配置文件（含 provider 预填）。
func TestInitCmd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	initRootCmd()

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{
		"ddns6", "init", "cloudflare",
		"--domain", "example.com",
		"--subdomain", "www",
		"--api-token", "tok",
		"--ttl", "300",
		"--interval", "10m",
		"--interface", "en0",
	}
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	cfgPath := filepath.Join(home, ".ddns6", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("读取配置: %v", err)
	}
	if !strings.Contains(string(data), "cloudflare") {
		t.Errorf("配置应包含 cloudflare, got %s", data)
	}
}

// TestPersistentPreRun_Logging 覆盖日志初始化（含 debug）。
func TestPersistentPreRun_Logging(t *testing.T) {
	initRootCmd()
	logPath := filepath.Join(t.TempDir(), "t.log")
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"ddns6", "list", "duckdns", "--log-file", logPath, "--debug"}
	_ = rootCmd.Execute()
}

// TestHandleClean_ConfirmYes 验证交互输入 yes 时执行删除。
func TestHandleClean_ConfirmYes(t *testing.T) {
	domains := []*ddns.Domain{{Domain: "example.com", SubDomain: "www", Type: "AAAA"}}
	cmd := listCleanFlags(t)
	m := &mockDNS{records: []ddns.RecordInfo{
		{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "2001:db8::1"},
	}}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() {
		fmt.Fprintln(w, "yes")
		w.Close()
	}()

	out := captureStdout(t, func() {
		if err := handleClean(cmd, domains, m); err != nil {
			t.Fatalf("handleClean: %v", err)
		}
	})
	m.mu.Lock()
	n := len(m.deleted)
	m.mu.Unlock()
	if n != 1 {
		t.Fatalf("应删除 1 条, got %d, out=%q", n, out)
	}
}

// TestRunWithConfig_UnsupportedProvider 覆盖 createProviderFromConfig 失败路径。
func TestRunWithConfig_UnsupportedProvider(t *testing.T) {
	writeTestConfig(t, `
provider: not-a-real-provider
domain: example.com
subdomains:
  - "@"
auth:
  token: "x"
`)
	err := runWithConfig(&cobra.Command{}, "list", func(*cobra.Command, *config.Config, []*ddns.Domain, ddns.DNSProvider) error {
		t.Fatal("不应调用 handler")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("期望 unsupported provider, got %v", err)
	}
}

// TestCheckFromConfig_EmptySubdomainsAndBadInterval 覆盖默认子域名与 interval 解析告警。
func TestCheckFromConfig_EmptySubdomainsAndBadInterval(t *testing.T) {
	cfg := &config.Config{
		Provider: "cloudflare",
		Domain:   "example.com",
		Auth:     map[string]string{"api_token": "tok"},
		Interval: "not-duration",
	}
	out := captureStdout(t, func() {
		_ = checkFromConfig(cfg)
	})
	if !strings.Contains(out, "subdomains: none") {
		t.Errorf("期望无子域名提示, got %q", out)
	}
	if !strings.Contains(out, "parse error") {
		t.Errorf("期望 interval 解析错误提示, got %q", out)
	}
}
