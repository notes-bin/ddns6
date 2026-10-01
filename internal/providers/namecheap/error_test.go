package namecheap

import (
	"net/http"
	"strings"
	"testing"
)

// TestHTTPStatusError_Error 验证错误字符串包含状态码与正文。
func TestHTTPStatusError_Error(t *testing.T) {
	t.Parallel()
	err := &httpStatusError{status: http.StatusForbidden, body: "denied"}
	got := err.Error()
	if !strings.Contains(got, "403") || !strings.Contains(got, "denied") {
		t.Fatalf("Error() = %q", got)
	}
}
