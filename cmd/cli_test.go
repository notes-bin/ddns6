package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestPrintVersion 验证版本输出包含 Version/Commit/BuildAt。
func TestPrintVersion(t *testing.T) {
	out := captureStdout(t, printVersion)
	for _, want := range []string{"Version:", "Commit:", "BuildAt:"} {
		if !strings.Contains(out, want) {
			t.Errorf("printVersion 输出应包含 %q, got %q", want, out)
		}
	}
}

// TestApplyEnvOverrides 验证环境变量覆盖未在命令行设置的 flag。
func TestApplyEnvOverrides(t *testing.T) {
	initRootCmd()

	t.Setenv("DDNS6_DOMAIN", "env.example.com")
	t.Setenv("DDNS6_TTL", "300")
	t.Setenv("DDNS6_DEBUG", "true")
	t.Setenv("DDNS6_INTERVAL", "7m")
	t.Setenv("DDNS6_SUBDOMAIN", "api")
	t.Setenv("DDNS6_INTERFACE", "utun0")
	t.Setenv("DDNS6_LOG_FILE", "test.log")

	// 确保这些 flag 视为未由命令行更改
	for _, name := range []string{"domain", "ttl", "debug", "interval", "subdomain", "interface", "log-file"} {
		if f := rootCmd.PersistentFlags().Lookup(name); f != nil {
			f.Changed = false
		}
	}

	applyEnvOverrides()

	if v, _ := rootCmd.PersistentFlags().GetString("domain"); v != "env.example.com" {
		t.Errorf("domain = %q, want env.example.com", v)
	}
	if v, _ := rootCmd.PersistentFlags().GetInt("ttl"); v != 300 {
		t.Errorf("ttl = %d, want 300", v)
	}
	if v, _ := rootCmd.PersistentFlags().GetBool("debug"); !v {
		t.Error("debug 应为 true")
	}
	if v, _ := rootCmd.PersistentFlags().GetString("interface"); v != "utun0" {
		t.Errorf("interface = %q", v)
	}
	if v, _ := rootCmd.PersistentFlags().GetString("log-file"); v != "test.log" {
		t.Errorf("log-file = %q", v)
	}
}

// TestApplyEnvOverrides_BoolFalseAndBadInt 覆盖 bool=false 与非法 int 忽略。
func TestApplyEnvOverrides_BoolFalseAndBadInt(t *testing.T) {
	initRootCmd()

	t.Setenv("DDNS6_DEBUG", "0")
	t.Setenv("DDNS6_TTL", "not-int")

	if f := rootCmd.PersistentFlags().Lookup("debug"); f != nil {
		f.Changed = false
		rootCmd.PersistentFlags().Set("debug", "true")
		f.Changed = false
	}
	if f := rootCmd.PersistentFlags().Lookup("ttl"); f != nil {
		rootCmd.PersistentFlags().Set("ttl", "600")
		f.Changed = false
	}

	applyEnvOverrides()

	if v, _ := rootCmd.PersistentFlags().GetBool("debug"); v {
		t.Error("DDNS6_DEBUG=0 后 debug 应为 false")
	}
	if v, _ := rootCmd.PersistentFlags().GetInt("ttl"); v != 600 {
		t.Errorf("非法 TTL 环境变量应被忽略, got %d", v)
	}
}

// TestExecute_Version 验证 CLI 入口 version 子命令。
func TestExecute_Version(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"ddns6", "version"}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	err = Execute()
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var buf bytes.Buffer
	io.Copy(&buf, r)
	if !strings.Contains(buf.String(), "Version:") {
		t.Errorf("version 输出异常: %q", buf.String())
	}
}

// TestExecute_UnknownCommand 验证未知命令打印帮助并返回 nil。
func TestExecute_UnknownCommand(t *testing.T) {
	initRootCmd()
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"ddns6", "not-a-real-command"}

	oldErr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	oldOut := os.Stdout
	os.Stdout = w

	err = Execute()
	w.Close()
	os.Stderr = oldErr
	os.Stdout = oldOut
	if err != nil {
		t.Fatalf("未知命令应返回 nil: %v", err)
	}
	var buf bytes.Buffer
	io.Copy(&buf, r)
	if !strings.Contains(buf.String(), "Error:") && !strings.Contains(buf.String(), "unknown") {
		// cobra 错误文案因版本略有差异，至少应有输出
		if buf.Len() == 0 {
			t.Error("未知命令应有错误/帮助输出")
		}
	}
}

// TestRegisteredCommands 验证 init 后 run/list/clean 已挂载运营商子命令。
func TestRegisteredCommands(t *testing.T) {
	initRootCmd()

	run := findCommand(rootCmd, "run")
	if run == nil {
		t.Fatal("缺少 run 命令")
	}
	if findCommand(run, "tencent") == nil {
		t.Error("run 应注册 tencent")
	}
	if findCommand(run, "duckdns") == nil {
		t.Error("run 应注册 duckdns")
	}

	list := findCommand(rootCmd, "list")
	if list == nil {
		t.Fatal("缺少 list 命令")
	}
	if findCommand(list, "cloudflare") == nil {
		t.Error("list 应注册 cloudflare")
	}
	// 受限运营商仍注册占位命令
	if findCommand(list, "duckdns") == nil {
		t.Error("list 应为 duckdns 注册受限占位命令")
	}

	clean := findCommand(rootCmd, "clean")
	if clean == nil {
		t.Fatal("缺少 clean 命令")
	}
	if findCommand(clean, "tencent") == nil {
		t.Error("clean 应注册 tencent")
	}

	// 执行受限 list 子命令，覆盖 registerRestrictedCommand 的 RunE
	duck := findCommand(list, "duckdns")
	err := duck.RunE(duck, nil)
	if err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("受限命令错误不符: %v", err)
	}
}

// findCommand 在 parent 的直接子命令中按名查找。
func findCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
