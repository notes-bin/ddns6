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

func TestSameHostRedirect_AllowsSameHost(t *testing.T) {
	t.Parallel()
	orig, _ := url.Parse("https://api.example.com/v1")
	next, _ := url.Parse("https://api.example.com/v2")
	err := SameHostRedirect(&http.Request{URL: next}, []*http.Request{{URL: orig}})
	if err != nil {
		t.Fatalf("同主机重定向应允许: %v", err)
	}
}

func TestSameHostRedirect_RejectsCrossHost(t *testing.T) {
	t.Parallel()
	orig, _ := url.Parse("https://api.example.com/v1")
	next, _ := url.Parse("http://169.254.169.254/")
	err := SameHostRedirect(&http.Request{URL: next}, []*http.Request{{URL: orig}})
	if err == nil {
		t.Fatal("跨主机重定向应拒绝")
	}
}

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
