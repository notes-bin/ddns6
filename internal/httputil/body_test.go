package httputil

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadBody_OK(t *testing.T) {
	got, err := ReadBody(strings.NewReader(`{"ok":true}`))
	if err != nil {
		t.Fatalf("ReadBody: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("got %q", got)
	}
}

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

func TestReadIPBody_ExceedsLimit(t *testing.T) {
	big := bytes.Repeat([]byte("1"), int(MaxIPResponseBytes)+1)
	_, err := ReadIPBody(bytes.NewReader(big))
	if err == nil {
		t.Fatal("expected error for oversized ip body")
	}
}

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

func TestReadBody_PropagatesReaderError(t *testing.T) {
	_, err := ReadBody(errReader{})
	if !errors.Is(err, errBoom) {
		t.Fatalf("got %v, want %v", err, errBoom)
	}
}

var errBoom = errors.New("boom")

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errBoom }
