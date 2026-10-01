package azure

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestHTTPStatusError_Error 验证错误字符串包含状态码与正文。
func TestHTTPStatusError_Error(t *testing.T) {
	t.Parallel()
	err := &httpStatusError{status: http.StatusBadRequest, body: "bad"}
	got := err.Error()
	if !strings.Contains(got, "400") || !strings.Contains(got, "bad") {
		t.Fatalf("Error() = %q", got)
	}
}

// TestIsNotFound 覆盖 404、非 404 与非 httpStatusError。
func TestIsNotFound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "404", err: &httpStatusError{status: http.StatusNotFound}, want: true},
		{name: "403", err: &httpStatusError{status: http.StatusForbidden}, want: false},
		{name: "普通错误", err: errors.New("boom"), want: false},
		{name: "nil", err: nil, want: false},
		{name: "包装 404", err: fmt.Errorf("wrap: %w", &httpStatusError{status: http.StatusNotFound}), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isNotFound(tt.err); got != tt.want {
				t.Errorf("isNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}
