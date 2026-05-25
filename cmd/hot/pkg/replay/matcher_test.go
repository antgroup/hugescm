// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package replay

import (
	"testing"

	"code.alipay.com/zeta/zeta/modules/git/gitobj"
)

// TestEqualerMatch verifies that the equaler matcher matches only on
// exact path equality.
func TestEqualerMatch(t *testing.T) {
	m := NewEqualer([]string{"a/b.txt", "c/d/e.bin"})
	cases := []struct {
		path string
		want bool
	}{
		{"a/b.txt", true},
		{"c/d/e.bin", true},
		{"a/b.txt.bak", false}, // equaler does not perform prefix matching
		{"a/b", false},
		{"", false},
	}
	for _, c := range cases {
		got := m.Match(&gitobj.TreeEntry{Name: c.path}, c.path)
		if got != c.want {
			t.Fatalf("equaler match(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestMatcherPrefix covers prefix matching semantics: a hit only when the
// path is exactly the prefix or sits on a directory boundary.
func TestMatcherPrefix(t *testing.T) {
	m := NewMatcher([]string{"vendor", "build/output/"})
	cases := []struct {
		path string
		want bool
	}{
		{"vendor", true},
		{"vendor/foo.go", true},
		{"vendor2", false}, // must end on a directory boundary
		{"build/output", true},
		{"build/output/x.bin", true},
		{"build/outputs", false}, // ditto
		{"src/main.go", false},
	}
	for _, c := range cases {
		got := m.Match(&gitobj.TreeEntry{Name: c.path}, c.path)
		if got != c.want {
			t.Fatalf("matcher prefix Match(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestMatcherWildcard covers wildcard matching.
func TestMatcherWildcard(t *testing.T) {
	m := NewMatcher([]string{"**/*.log", "node_modules"})
	cases := []struct {
		path string
		want bool
	}{
		{"a/b/c.log", true},
		{"x.log", true},
		{"a/b/c.txt", false},
		{"node_modules", true},
		{"node_modules/foo/bar.js", true},
	}
	for _, c := range cases {
		got := m.Match(&gitobj.TreeEntry{Name: c.path}, c.path)
		if got != c.want {
			t.Fatalf("matcher wildcard Match(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestMatcherEmpty: an empty pattern set must match everything, matching
// the contract expected by the drop/graft callers.
func TestMatcherEmpty(t *testing.T) {
	m := NewMatcher(nil)
	if !m.Match(&gitobj.TreeEntry{Name: "anything"}, "anything") {
		t.Fatal("empty matcher should match everything")
	}
}
