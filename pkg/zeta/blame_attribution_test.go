// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// TestBlameAttribution pins the line-attribution invariants of the blame
// walk in a throwaway repository, covering the two historical bugs:
//
//  1. a `continue out` mislabel made the walk abandon the remainder of a
//     hunk after the first deletion, so lines after a deletion were
//     misattributed to the newest commit (pinned by commit B: a pure
//     deletion must not own any surviving line);
//  2. the walk used hunk context that hid unchanged lines between hunks
//     and desynced the prevl/curl counters (pinned by commit C touching
//     two clusters more than 2*DefaultContextLines apart: the gap lines
//     and everything after the second cluster must stay attributed to A).
//
// It replaces blind trust in the developer-only TestBlame below, which
// silently passes when its local fixture is missing.
func TestBlameAttribution(t *testing.T) {
	ctx := context.Background()
	// Resolve the temp dir physically: on macOS /var is a symlink and the
	// test binary's Getwd falls back to the physical path when $PWD does
	// not match the new cwd, so Init's worktree and the process cwd must
	// agree for pathspec resolution to see files as inside the repo.
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

	const fname = "file.txt"
	commitLines := func(name string, lines []string) plumbing.Hash {
		t.Helper()
		// Worktree.Commit resolves the commit identity at COMMIT TIME from
		// the repository config (user.name / user.email) with the
		// ZETA_AUTHOR_NAME / ZETA_AUTHOR_EMAIL environment variables as
		// fallback (see Repository.authorName/authorEmail) - the Author
		// field of CommitOptions is NOT honored. Neither source exists on
		// a clean machine, so set the env explicitly per commit; it is
		// read lazily, so every commit gets its own author identity.
		t.Setenv(ENV_ZETA_AUTHOR_NAME, name)
		t.Setenv(ENV_ZETA_AUTHOR_EMAIL, strings.ToLower(name)+"@example.com")
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(dir, fname), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", fname, err)
		}
		if err := w.Add(ctx, []string{fname}, false); err != nil {
			t.Fatalf("add %s: %v", fname, err)
		}
		h, err := w.Commit(ctx, &CommitOptions{
			Message: []string{fmt.Sprintf("commit by %s", name)},
		})
		if err != nil {
			t.Fatalf("commit %s: %v", name, err)
		}
		return h
	}

	lines := func(prefix string, n int) []string {
		out := make([]string, 0, n)
		for i := 1; i <= n; i++ {
			out = append(out, fmt.Sprintf("%s%d", prefix, i))
		}
		return out
	}

	// Commit A: file with 16 lines.
	a := commitLines("alice", lines("l", 16))

	// Commit B: delete the middle line l8 (a pure deletion owning nothing).
	bLines := lines("l", 16)
	bLines = append(bLines[:7], bLines[8:]...) // drop l8 (index 7)
	bHash := commitLines("bob", bLines)

	// Commit C: modify l2 and l14 in the same commit. The two change
	// clusters sit 11 lines apart, farther than 2*DefaultContextLines, so
	// the default-context diff would split them into two hunks (the
	// historical desync trigger).
	cLines := append([]string(nil), bLines...)
	cLines[1] = "l2 CHANGED"
	cLines[12] = "l14 CHANGED"
	cHash := commitLines("carol", cLines)

	result, err := r.BlameFile(ctx, "", fname)
	if err != nil {
		t.Fatalf("blame: %v", err)
	}
	if len(result.Lines) != len(cLines) {
		t.Fatalf("blame returned %d lines, want %d", len(result.Lines), len(cLines))
	}

	// Expected attribution for every surviving line of the B→C content.
	want := make([]plumbing.Hash, len(cLines))
	for i := range want {
		want[i] = a
	}
	want[1] = cHash  // l2 CHANGED
	want[12] = cHash // l14 CHANGED

	for i, line := range result.Lines {
		which := map[plumbing.Hash]string{a: "A", bHash: "B", cHash: "C"}[line.Hash]
		if want[i] == a && (line.Hash == bHash || line.Hash == cHash) {
			t.Errorf("line %d (%q) attributed to commit %s (%s), want the line's introducing commit A", i+1, line.Text, which, line.Hash)
			continue
		}
		if line.Hash != want[i] {
			t.Errorf("line %d (%q) attributed to commit %s (%s), want %s", i+1, line.Text, which, line.Hash, map[plumbing.Hash]string{a: "A", bHash: "B", cHash: "C"}[want[i]])
		}
		// Invariant (1): the pure-deletion commit B must own no line.
		if line.Hash == bHash {
			t.Errorf("line %d (%q) attributed to the pure-deletion commit B", i+1, line.Text)
		}
	}

	// Invariant (2) in the words of the bug reports: lines after the first
	// change of a file must not be swept into the newest commit. Lines
	// after the l8 deletion (l9..l13) and after the second change cluster
	// (l15, l16) must both still be owned by A.
	t.Run("deletion aftermath keeps original attribution", func(t *testing.T) {
		texts := make([]string, len(result.Lines))
		for i, l := range result.Lines {
			texts[i] = l.Text
		}
		for _, wantText := range []string{"l9", "l10", "l11", "l12", "l13", "l15", "l16"} {
			idx := indexOf(texts, wantText)
			if idx < 0 {
				t.Fatalf("line %q missing from blame result", wantText)
			}
			if h := result.Lines[idx].Hash; h != a {
				t.Errorf("line %q attributed to %s, want introducing commit A", wantText, h)
			}
		}
	})
}

func indexOf(ss []string, v string) int {
	for i, s := range ss {
		if s == v {
			return i
		}
	}
	return -1
}
