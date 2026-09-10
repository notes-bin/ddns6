package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// TestPrintVersion 验证版本输出包含 Version/Commit/BuildAt。
func TestPrintVersion(t *testing.T) {
	out := captureStdout(t, printVersion)
	requireContains(t, out, "Version:", "Commit:", "BuildAt:")
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

	for _, name := range []string{"domain", "ttl", "debug", "interval", "subdomain", "interface", "log-file"} {
		if f := rootCmd.PersistentFlags().Lookup(name); f != nil {
			f.Changed = false
		}
	}

	applyEnvOverrides()

	checks := []struct {
		name string
		want any
	}{
		{"domain", "env.example.com"},
		{"ttl", 300},
		{"debug", true},
		{"interface", "utun0"},
		{"log-file", "test.log"},
	}
	for _, c := range checks {
		switch want := c.want.(type) {
		case string:
			if v, _ := rootCmd.PersistentFlags().GetString(c.name); v != want {
				t.Errorf("%s = %q, want %q", c.name, v, want)
			}
		case int:
			if v, _ := rootCmd.PersistentFlags().GetInt(c.name); v != want {
				t.Errorf("%s = %d, want %d", c.name, v, want)
			}
		case bool:
			if v, _ := rootCmd.PersistentFlags().GetBool(c.name); v != want {
				t.Errorf("%s = %v, want %v", c.name, v, want)
			}
		}
	}
}

// TestApplyEnvOverrides_BoolFalseAndBadInt 覆盖 bool=false 与非法 int 忽略。
func TestApplyEnvOverrides_BoolFalseAndBadInt(t *testing.T) {
	initRootCmd()

	t.Setenv("DDNS6_DEBUG", "0")
	t.Setenv("DDNS6_TTL", "not-int")

	if f := rootCmd.PersistentFlags().Lookup("debug"); f != nil {
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
	withArgs(t, "ddns6", "version")
	out := captureStdout(t, func() {
		if err := Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	requireContains(t, out, "Version:")
}

// TestExecute_UnknownCommand 验证未知命令打印帮助并返回 nil。
func TestExecute_UnknownCommand(t *testing.T) {
	initRootCmd()
	withArgs(t, "ddns6", "not-a-real-command")

	oldErr, oldOut := os.Stderr, os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr, os.Stdout = w, w
	err = Execute()
	w.Close()
	os.Stderr, os.Stdout = oldErr, oldOut
	if err != nil {
		t.Fatalf("未知命令应返回 nil: %v", err)
	}
	var buf bytes.Buffer
	io.Copy(&buf, r)
	if buf.Len() == 0 {
		t.Error("未知命令应有错误/帮助输出")
	}
}

// TestRegisteredCommands 验证 init 后 run/list/clean 已挂载运营商子命令。
func TestRegisteredCommands(t *testing.T) {
	initRootCmd()

	for _, tt := range []struct {
		parent, child string
	}{
		{"run", "tencent"},
		{"run", "duckdns"},
		{"list", "cloudflare"},
		{"list", "duckdns"},
		{"clean", "tencent"},
	} {
		parent := findCommand(rootCmd, tt.parent)
		if parent == nil {
			t.Fatalf("缺少 %s 命令", tt.parent)
		}
		if findCommand(parent, tt.child) == nil {
			t.Errorf("%s 应注册 %s", tt.parent, tt.child)
		}
	}

	duck := findCommand(findCommand(rootCmd, "list"), "duckdns")
	requireErrContains(t, duck.RunE(duck, nil), "does not support")
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
