// Package crypto 密码学工具函数测试
package crypto

import (
	"encoding/hex"
	"testing"
)

// h 辅助函数，将十六进制字符串解码为 []byte，测试中用。
func h(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) 失败: %v", s, err)
	}
	return b
}

// ============================================================
// HMACSHA256 / HMACSHA256Hex 测试 - RFC 4231 Test Vectors
// ============================================================

func TestHMACSHA256_RFC4231_Test1(t *testing.T) {
	// Test Case 1: key=20 bytes of 0x0b, data="Hi There"
	key := h(t, "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b")
	data := []byte("Hi There")
	expected := "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"

	got := HMACSHA256Hex(key, data)
	if got != expected {
		t.Errorf("HMACSHA256Hex 测试 1:\n  got:  %s\n  want: %s", got, expected)
	}

	raw := HMACSHA256(key, data)
	if hex.EncodeToString(raw) != expected {
		t.Errorf("HMACSHA256 测试 1:\n  got:  %x\n  want: %s", raw, expected)
	}
}

func TestHMACSHA256_RFC4231_Test2(t *testing.T) {
	// Test Case 2: key="Jefe", data="what do ya want for nothing?"
	key := []byte("Jefe")
	data := []byte("what do ya want for nothing?")
	expected := "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"

	got := HMACSHA256Hex(key, data)
	if got != expected {
		t.Errorf("HMACSHA256Hex 测试 2:\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestHMACSHA256_RFC4231_Test3(t *testing.T) {
	// RFC 4231 Test Case 3:
	// key = 20 bytes of 0xaa
	// data = 50 bytes of 0xdd
	key := h(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	data := h(t, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	expected := "773ea91e36800e46854db8ebd09181a72959098b3ef8c122d9635514ced565fe"

	got := HMACSHA256Hex(key, data)
	if got != expected {
		t.Errorf("HMACSHA256Hex 测试 3:\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestSHA256Hex_Simple(t *testing.T) {
	// 与 openssl dgst -sha256 输出对照
	data := []byte("hello world")
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	got := SHA256Hex(data)
	if got != expected {
		t.Errorf("SHA256Hex(\"hello world\") = %s, want %s", got, expected)
	}
}

// ============================================================
// SHA256Hex 测试
// ============================================================

func TestSHA256Hex_Basic(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "空字符串",
			data: []byte(""),
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "基本字符串",
			data: []byte("hello"),
			want: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SHA256Hex(tt.data)
			if got != tt.want {
				t.Errorf("SHA256Hex(%q) = %s, want %s", tt.data, got, tt.want)
			}
		})
	}
}
