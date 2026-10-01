package httputil

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// TestReadBody_OK 验证正常大小响应体可完整读取。
func TestReadBody_OK(t *testing.T) {
	got, err := ReadBody(strings.NewReader(`{"ok":true}`))
	if err != nil {
		t.Fatalf("ReadBody: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("got %q", got)
	}
}

// TestReadBody_ExceedsLimit 验证超过 MaxResponseBytes 时返回超限错误。
func TestReadBody_ExceedsLimit(t *testing.T) {
	big := bytes.Repeat([]byte("a"), int(MaxResponseBytes)+1)
	_, err := ReadBody(bytes.NewReader(big))
	if err == nil {
		t.Fatal("expected error for oversized body")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestReadIPBody_ExceedsLimit 验证超过 MaxIPResponseBytes 时返回超限错误。
func TestReadIPBody_ExceedsLimit(t *testing.T) {
	big := bytes.Repeat([]byte("1"), int(MaxIPResponseBytes)+1)
	_, err := ReadIPBody(bytes.NewReader(big))
	if err == nil {
		t.Fatal("expected error for oversized ip body")
	}
}

// TestLimitBody 验证 LimitBody 包装后可读出未超限内容。
func TestLimitBody(t *testing.T) {
	r := LimitBody(strings.NewReader("hello"))
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
}

// TestReadBody_PropagatesReaderError 验证底层 Reader 错误被原样向上返回。
func TestReadBody_PropagatesReaderError(t *testing.T) {
	_, err := ReadBody(errReader{})
	if !errors.Is(err, errBoom) {
		t.Fatalf("got %v, want %v", err, errBoom)
	}
}

var errBoom = errors.New("boom")

// errReader 始终返回固定错误，供传播测试使用。
type errReader struct{}

// Read 始终返回 errBoom。
func (errReader) Read([]byte) (int, error) { return 0, errBoom }
