package domainutil

import "testing"

func TestSplitDomain(t *testing.T) {
	tests := []struct {
		input      string
		rootDomain string
		wantRoot   string
		wantSub    string
	}{
		// 无 rootDomain（旧逻辑向后兼容）
		{"example.com", "", "example.com", "@"},
		{"www.example.com", "", "example.com", "www"},
		{"sub.www.example.com", "", "example.com", "sub.www"},

		// 有已知 rootDomain
		{"example.com", "example.com", "example.com", "@"},
		{"www.example.com", "example.com", "example.com", "www"},
		{"sub.www.example.com", "example.com", "example.com", "sub.www"},

		// 多部分 TLD
		{"example.co.uk", "example.co.uk", "example.co.uk", "@"},
		{"www.example.co.uk", "example.co.uk", "example.co.uk", "www"},
		{"sub.www.example.co.uk", "example.co.uk", "example.co.uk", "sub.www"},
	}
	for _, tt := range tests {
		root, sub := SplitDomain(tt.input, tt.rootDomain)
		if root != tt.wantRoot {
			t.Errorf("SplitDomain(%q, %q) root = %q, want %q", tt.input, tt.rootDomain, root, tt.wantRoot)
		}
		if sub != tt.wantSub {
			t.Errorf("SplitDomain(%q, %q) sub = %q, want %q", tt.input, tt.rootDomain, sub, tt.wantSub)
		}
	}
}
