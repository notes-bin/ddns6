package httputil

import (
	"errors"
	"strings"
	"testing"
)

// TestTruncateForLog 验证短串原样返回、长串截断并带 truncated 后缀。
func TestTruncateForLog(t *testing.T) {
	if got := TruncateForLog("short"); got != "short" {
		t.Fatalf("短串不应截断: %q", got)
	}
	long := strings.Repeat("a", MaxLogErrorBytes+50)
	got := TruncateForLog(long)
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Fatalf("应有 truncated 后缀: %q", got)
	}
	if len(got) > MaxLogErrorBytes+len("...(truncated)") {
		t.Fatalf("截断后过长: %d", len(got))
	}
}

// TestErrForLog 验证 nil 返回空串、普通错误返回其消息。
func TestErrForLog(t *testing.T) {
	if ErrForLog(nil) != "" {
		t.Fatal("nil 应返回空串")
	}
	if got := ErrForLog(errors.New("boom")); got != "boom" {
		t.Fatalf("got %q", got)
	}
}
