package httputil

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"
)

// TestSameHostRedirect_AllowsSameHost 验证同 scheme+host 重定向被允许。
func TestSameHostRedirect_AllowsSameHost(t *testing.T) {
	t.Parallel()
	orig, _ := url.Parse("https://api.example.com/v1")
	next, _ := url.Parse("https://api.example.com/v2")
	err := SameHostRedirect(&http.Request{URL: next}, []*http.Request{{URL: orig}})
	if err != nil {
		t.Fatalf("同主机重定向应允许: %v", err)
	}
}

// TestSameHostRedirect_RejectsCrossHost 验证跨主机重定向被拒绝。
func TestSameHostRedirect_RejectsCrossHost(t *testing.T) {
	t.Parallel()
	orig, _ := url.Parse("https://api.example.com/v1")
	next, _ := url.Parse("http://169.254.169.254/")
	err := SameHostRedirect(&http.Request{URL: next}, []*http.Request{{URL: orig}})
	if err == nil {
		t.Fatal("跨主机重定向应拒绝")
	}
}

// TestNewHTTPClient_HasTimeoutAndRedirect 验证客户端超时与 SameHostRedirect 已设置。
func TestNewHTTPClient_HasTimeoutAndRedirect(t *testing.T) {
	t.Parallel()
	c := NewHTTPClient(3 * time.Second)
	if c.Timeout != 3*time.Second {
		t.Fatalf("Timeout=%v", c.Timeout)
	}
	if c.CheckRedirect == nil {
		t.Fatal("CheckRedirect 未设置")
	}
}

// TestRedactSecrets_QueryParams 验证 URL query 中的 token 等密钥被替换为 REDACTED。
func TestRedactSecrets_QueryParams(t *testing.T) {
	t.Parallel()
	in := `request failed: Get "https://www.duckdns.org/update?token=secret123&domains=x": EOF`
	got := RedactSecrets(in)
	if !regexp.MustCompile(`token=REDACTED`).MatchString(got) {
		t.Fatalf("token 未脱敏: %s", got)
	}
	if regexp.MustCompile(`secret123`).MatchString(got) {
		t.Fatalf("明文仍在: %s", got)
	}
}

// TestSanitizeError_PreservesUnwrap 验证脱敏后仍保留 errors.Is 可追踪的 Unwrap 链。
func TestSanitizeError_PreservesUnwrap(t *testing.T) {
	t.Parallel()
	base := errors.New("root")
	wrapped := fmt.Errorf(`duckdns: Get "https://x/?token=abc": %w`, base)
	got := SanitizeError(wrapped)
	if !errors.Is(got, base) {
		t.Fatal("应保留 Unwrap 链")
	}
	if regexp.MustCompile(`token=abc`).MatchString(got.Error()) {
		t.Fatalf("未脱敏: %v", got)
	}
}
