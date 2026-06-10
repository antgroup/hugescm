// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"testing"

	"code.alipay.com/zeta/zeta/modules/plumbing"
	"code.alipay.com/zeta/zeta/modules/plumbing/filemode"
	"code.alipay.com/zeta/zeta/modules/zeta/object"
)

// TestNameConflictsFirstMatch verifies that nameConflicts() records the first
// matching path (sorted order) when a file name is a prefix of multiple paths.
func TestNameConflictsFirstMatch(t *testing.T) {
	d := &differences{
		entries: map[string]*ChangeEntry{
			"a":   {Path: "a", Our: &object.TreeEntry{Name: "a", Mode: filemode.Regular, Hash: plumbing.NewHash("1111111111111111111111111111111111111111111111111111111111111111")}},
			"a/b": {Path: "a/b", Their: &object.TreeEntry{Name: "b", Mode: filemode.Regular, Hash: plumbing.NewHash("2222222222222222222222222222222222222222222222222222222222222222")}},
			"a/c": {Path: "a/c", Their: &object.TreeEntry{Name: "c", Mode: filemode.Regular, Hash: plumbing.NewHash("3333333333333333333333333333333333333333333333333333333333333333")}},
		},
		renames: make(map[string]*RenameEntry),
		ours:    map[string]bool{"a": true},
		theirs:  map[string]bool{"a/b": true, "a/c": true},
	}

	conflicts := d.nameConflicts()

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict entry, got %d", len(conflicts))
	}

	// After fix: records the first matching path in sorted order ("a/b" < "a/c").
	val, ok := conflicts["a"]
	if !ok {
		t.Fatal("expected conflict for key 'a'")
	}

	if val != "a/b" {
		t.Errorf("expected conflict value 'a/b' (first sorted match), got %q", val)
	}
}

// TestHasConflictCorrectness verifies that hasConflict() logic is correct.
func TestHasConflictCorrectness(t *testing.T) {
	hash1 := plumbing.NewHash("1111111111111111111111111111111111111111111111111111111111111111")
	hash2 := plumbing.NewHash("2222222222222222222222222222222222222222222222222222222222222222")
	hash3 := plumbing.NewHash("3333333333333333333333333333333333333333333333333333333333333333")

	tests := []struct {
		name     string
		entry    ChangeEntry
		expected bool
	}{
		{
			name: "ancestor==ours, theirs modified (no conflict - fast forward theirs)",
			entry: ChangeEntry{
				Path:     "file.txt",
				Ancestor: &object.TreeEntry{Name: "file.txt", Hash: hash1, Mode: filemode.Regular},
				Our:      &object.TreeEntry{Name: "file.txt", Hash: hash1, Mode: filemode.Regular},
				Their:    &object.TreeEntry{Name: "file.txt", Hash: hash2, Mode: filemode.Regular},
			},
			expected: false,
		},
		{
			name: "ancestor==theirs, ours modified (no conflict - fast forward ours)",
			entry: ChangeEntry{
				Path:     "file.txt",
				Ancestor: &object.TreeEntry{Name: "file.txt", Hash: hash1, Mode: filemode.Regular},
				Our:      &object.TreeEntry{Name: "file.txt", Hash: hash2, Mode: filemode.Regular},
				Their:    &object.TreeEntry{Name: "file.txt", Hash: hash1, Mode: filemode.Regular},
			},
			expected: false,
		},
		{
			name: "ours==theirs, both modified same way (no conflict)",
			entry: ChangeEntry{
				Path:     "file.txt",
				Ancestor: &object.TreeEntry{Name: "file.txt", Hash: hash1, Mode: filemode.Regular},
				Our:      &object.TreeEntry{Name: "file.txt", Hash: hash2, Mode: filemode.Regular},
				Their:    &object.TreeEntry{Name: "file.txt", Hash: hash2, Mode: filemode.Regular},
			},
			expected: false,
		},
		{
			name: "all three different (conflict)",
			entry: ChangeEntry{
				Path:     "file.txt",
				Ancestor: &object.TreeEntry{Name: "file.txt", Hash: hash1, Mode: filemode.Regular},
				Our:      &object.TreeEntry{Name: "file.txt", Hash: hash2, Mode: filemode.Regular},
				Their:    &object.TreeEntry{Name: "file.txt", Hash: hash3, Mode: filemode.Regular},
			},
			expected: true,
		},
		{
			name: "ours deleted (nil), theirs modified (conflict)",
			entry: ChangeEntry{
				Path:     "file.txt",
				Ancestor: &object.TreeEntry{Name: "file.txt", Hash: hash1, Mode: filemode.Regular},
				Our:      nil,
				Their:    &object.TreeEntry{Name: "file.txt", Hash: hash2, Mode: filemode.Regular},
			},
			expected: true,
		},
		{
			name: "both deleted (nil) (no conflict - both agree)",
			entry: ChangeEntry{
				Path:     "file.txt",
				Ancestor: &object.TreeEntry{Name: "file.txt", Hash: hash1, Mode: filemode.Regular},
				Our:      nil,
				Their:    nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.hasConflict()
			if got != tt.expected {
				t.Errorf("hasConflict() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestNameConflictsBasic verifies nameConflicts() works for simple cases.
func TestNameConflictsBasic(t *testing.T) {
	tests := []struct {
		name     string
		entries  map[string]*ChangeEntry
		expected map[string]string
	}{
		{
			name: "no conflicts",
			entries: map[string]*ChangeEntry{
				"a.txt": {Path: "a.txt"},
				"b.txt": {Path: "b.txt"},
			},
			expected: map[string]string{},
		},
		{
			name: "simple file/directory conflict",
			entries: map[string]*ChangeEntry{
				"a":     {Path: "a"},
				"a/b":   {Path: "a/b"},
				"c.txt": {Path: "c.txt"},
			},
			expected: map[string]string{"a": "a/b"},
		},
		{
			name: "multiple conflicts under same prefix (first match recorded)",
			entries: map[string]*ChangeEntry{
				"dir":     {Path: "dir"},
				"dir/x":   {Path: "dir/x"},
				"dir/y":   {Path: "dir/y"},
				"dir/z":   {Path: "dir/z"},
				"other":   {Path: "other"},
				"other/w": {Path: "other/w"},
			},
			// After fix: records the first sorted match for each prefix
			expected: map[string]string{
				"dir":   "dir/x",   // first sorted match
				"other": "other/w", // only one match
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &differences{
				entries: tt.entries,
				renames: make(map[string]*RenameEntry),
				ours:    make(map[string]bool),
				theirs:  make(map[string]bool),
			}
			got := d.nameConflicts()
			if len(got) != len(tt.expected) {
				t.Errorf("nameConflicts() returned %d entries, want %d\n  got: %v\n  want: %v",
					len(got), len(tt.expected), got, tt.expected)
				return
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("nameConflicts()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
