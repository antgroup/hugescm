// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestAddAbsolutePathspec is the regression test for `zeta add` rejecting
// absolute pathspecs that point INSIDE the repository: they used to be
// joined onto the process cwd (filepath.Join(cwd, "/abs/...") concatenates
// instead of replacing), so every absolute pathspec failed with
// "is outside repository" while the error message displayed the original
// (in-repo) path. git, by contrast, accepts in-repo absolute pathspecs.
func TestAddAbsolutePathspec(t *testing.T) {
	ctx := context.Background()
	raw := t.TempDir()
	dir, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("resolve temp dir %s: %v", raw, err)
	}
	t.Chdir(dir)

	r, err := Init(ctx, &InitOptions{Branch: "mainline", Worktree: dir})
	if err != nil {
		t.Fatalf("init repository: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	w := r.Worktree()

	const fname = "abs.txt"
	if err := os.WriteFile(filepath.Join(dir, fname), []byte("absolute pathspec\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", fname, err)
	}

	// Absolute pathspec pointing inside the repository must be accepted
	// and must stage the root-relative entry.
	if err := w.Add(ctx, []string{filepath.Join(dir, fname)}, false); err != nil {
		t.Fatalf("add with absolute pathspec: %v", err)
	}
	idx, err := w.odb.Index()
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if _, err := idx.Entry(fname); err != nil {
		t.Fatalf("index must contain the root-relative entry %q after add: %v", fname, err)
	}
}
