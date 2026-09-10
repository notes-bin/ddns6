package cmd

import (
	"fmt"
	"strings"
	"testing"
)

// TestFormatProviderList 验证运营商表格含全部工厂名且受限为 no。
func TestFormatProviderList(t *testing.T) {
	out := formatProviderList(providerFactories)
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "RECORDS/CLEAN") {
		t.Fatalf("missing header: %s", out)
	}
	for _, p := range providerFactories {
		if !strings.Contains(out, p.name) {
			t.Errorf("missing provider %q", p.name)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "duckdns" && fields[1] != "no" {
			t.Errorf("duckdns 应标记为 no: %q", line)
		}
		if len(fields) >= 2 && fields[0] == "tencent" && fields[1] != "yes" {
			t.Errorf("tencent 应标记为 yes: %q", line)
		}
	}
	if !strings.Contains(out, fmt.Sprintf("Total: %d providers", len(providerFactories))) {
		t.Errorf("missing total line: %s", out)
	}
}

// TestListCmd_RejectExtraArgs 验证 list 拒绝多余 positional 参数（旧 list provider 用法）。
func TestListCmd_RejectExtraArgs(t *testing.T) {
	initRootCmd()
	withArgs(t, "ddns6", "list", "tencent")
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("list tencent 应返回错误")
	}
}

// TestListCmd_Execute 验证 ddns6 list 成功且输出含 tencent。
func TestListCmd_Execute(t *testing.T) {
	initRootCmd()
	out := captureStdout(t, func() {
		withArgs(t, "ddns6", "list")
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("list: %v", err)
		}
	})
	if !strings.Contains(out, "tencent") {
		t.Errorf("missing tencent in output: %s", out)
	}
}
