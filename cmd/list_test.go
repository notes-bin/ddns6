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
	if !strings.Contains(out, "duckdns") || !strings.Contains(out, "no") {
		t.Error("expected duckdns marked no for RECORDS/CLEAN")
	}
	if !strings.Contains(out, fmt.Sprintf("Total: %d providers", len(providerFactories))) {
		t.Errorf("missing total line: %s", out)
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
