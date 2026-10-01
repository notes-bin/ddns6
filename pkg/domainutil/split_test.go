package domainutil

import "testing"

// TestSplitDomain 覆盖无/有 rootDomain、多部分 TLD、apex（@）及单段主机名场景。
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

// FuzzSplitDomain 对任意输入执行 SplitDomain，确保不 panic，
// 且非空输入时 root 非空、apex 场景结果自洽。
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

// TestZoneCandidates 覆盖空输入、尾点、单段、apex、多层子域与多部分 TLD；
// 确认不含单段纯 TLD，但会保留两段后缀（如 co.uk）。
func TestZoneCandidates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "空", input: "", want: nil},
		{name: "点", input: ".", want: nil},
		{name: "单段", input: "localhost", want: []string{"localhost"}},
		{name: "apex", input: "example.com", want: []string{"example.com"}},
		{name: "www", input: "www.example.com", want: []string{"www.example.com", "example.com"}},
		{name: "多层", input: "a.b.example.com", want: []string{"a.b.example.com", "b.example.com", "example.com"}},
		{name: "co_uk", input: "www.example.co.uk", want: []string{"www.example.co.uk", "example.co.uk", "co.uk"}},
		{name: "尾点", input: "www.example.com.", want: []string{"www.example.com", "example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ZoneCandidates(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ZoneCandidates(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ZoneCandidates(%q) = %v, want %v", tt.input, got, tt.want)
				}
			}
		})
	}
}
