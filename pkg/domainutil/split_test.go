package domainutil

import "testing"

func TestSplitDomain(t *testing.T) {
	tests := []struct {
		input    string
		wantRoot string
		wantSub  string
	}{
		{"example.com", "example.com", "@"},
		{"www.example.com", "example.com", "www"},
		{"sub.www.example.com", "example.com", "sub.www"},
	}
	for _, tt := range tests {
		root, sub := SplitDomain(tt.input)
		if root != tt.wantRoot {
			t.Errorf("SplitDomain(%q) root = %q, want %q", tt.input, root, tt.wantRoot)
		}
		if sub != tt.wantSub {
			t.Errorf("SplitDomain(%q) sub = %q, want %q", tt.input, sub, tt.wantSub)
		}
	}
}
