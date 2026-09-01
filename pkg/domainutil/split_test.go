package domainutil

import "testing"

// TestSplitDomain 覆盖无/有 rootDomain、多部分 TLD 及 apex（@）场景。
func TestSplitDomain(t *testing.T) {
	tests := []struct {
		input      string
		rootDomain string
		wantRoot   string
		wantSub    string
	}{
		{"example.com", "", "example.com", "@"},
		{"www.example.com", "", "example.com", "www"},
		{"sub.www.example.com", "", "example.com", "sub.www"},

		{"example.com", "example.com", "example.com", "@"},
		{"www.example.com", "example.com", "example.com", "www"},
		{"sub.www.example.com", "example.com", "example.com", "sub.www"},

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
