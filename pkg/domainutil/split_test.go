package domainutil

import "testing"

// TestSplitDomain 覆盖无/有 rootDomain、多部分 TLD 及 apex（@）场景。
func TestSplitDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		rootDomain string
		wantRoot   string
		wantSub    string
	}{
		{name: "无根_apex", input: "example.com", rootDomain: "", wantRoot: "example.com", wantSub: "@"},
		{name: "无根_www", input: "www.example.com", rootDomain: "", wantRoot: "example.com", wantSub: "www"},
		{name: "无根_多层", input: "sub.www.example.com", rootDomain: "", wantRoot: "example.com", wantSub: "sub.www"},

		{name: "有根_apex", input: "example.com", rootDomain: "example.com", wantRoot: "example.com", wantSub: "@"},
		{name: "有根_www", input: "www.example.com", rootDomain: "example.com", wantRoot: "example.com", wantSub: "www"},
		{name: "有根_多层", input: "sub.www.example.com", rootDomain: "example.com", wantRoot: "example.com", wantSub: "sub.www"},

		{name: "co_uk_apex", input: "example.co.uk", rootDomain: "example.co.uk", wantRoot: "example.co.uk", wantSub: "@"},
		{name: "co_uk_www", input: "www.example.co.uk", rootDomain: "example.co.uk", wantRoot: "example.co.uk", wantSub: "www"},
		{name: "co_uk_多层", input: "sub.www.example.co.uk", rootDomain: "example.co.uk", wantRoot: "example.co.uk", wantSub: "sub.www"},

		{name: "单段无根", input: "localhost", rootDomain: "", wantRoot: "localhost", wantSub: "@"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root, sub := SplitDomain(tt.input, tt.rootDomain)
			if root != tt.wantRoot {
				t.Errorf("SplitDomain(%q, %q) root = %q, want %q", tt.input, tt.rootDomain, root, tt.wantRoot)
			}
			if sub != tt.wantSub {
				t.Errorf("SplitDomain(%q, %q) sub = %q, want %q", tt.input, tt.rootDomain, sub, tt.wantSub)
			}
		})
	}
}

// FuzzSplitDomain 对任意输入执行 SplitDomain，确保不 panic 且根域名非空时结果自洽。
func FuzzSplitDomain(f *testing.F) {
	f.Add("www.example.com", "example.com")
	f.Add("example.com", "")
	f.Add("a.b.example.co.uk", "example.co.uk")
	f.Add("", "")
	f.Add("localhost", "")

	f.Fuzz(func(t *testing.T, fulldomain, rootDomain string) {
		root, sub := SplitDomain(fulldomain, rootDomain)
		if root == "" && fulldomain != "" {
			t.Fatalf("非空输入时 root 不应为空: fulldomain=%q rootDomain=%q", fulldomain, rootDomain)
		}
		_ = sub
		if rootDomain != "" && fulldomain == rootDomain {
			if root != rootDomain || sub != "@" {
				t.Fatalf("apex: got (%q,%q)", root, sub)
			}
		}
	})
}
