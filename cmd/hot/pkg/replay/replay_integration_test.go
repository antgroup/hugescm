// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package replay

import (
	"context"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"code.alipay.com/zeta/zeta/modules/git"
	"code.alipay.com/zeta/zeta/modules/git/gitobj"
)

// requireGit skips environments where the git CLI is unavailable.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH; skip integration test")
	}
}

// makeBareRepo creates a bare repository and returns its path and an open ODB.
func makeBareRepo(t *testing.T) (string, *git.ODB) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo.git")
	if err := git.NewRepo(context.Background(), dir, "main", true, git.HashSHA1); err != nil {
		t.Fatalf("init bare repo: %v", err)
	}
	odb, err := git.NewODB(dir, git.HashSHA1)
	if err != nil {
		t.Fatalf("open odb: %v", err)
	}
	t.Cleanup(func() { _ = odb.Close() })
	return dir, odb
}

// writeBlob stores body as a blob and returns the resulting oid.
func writeBlob(t *testing.T, odb *git.ODB, body string) []byte {
	t.Helper()
	r := strings.NewReader(body)
	oid, err := odb.WriteBlob(&gitobj.Blob{
		Size:     int64(len(body)),
		Contents: r,
	})
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	return oid
}

// writeCommit writes a commit object directly via the ODB, bypassing index/worktree.
func writeCommit(t *testing.T, odb *git.ODB, treeOID []byte, parents [][]byte, msg string) []byte {
	t.Helper()
	oid, err := odb.WriteCommit(&gitobj.Commit{
		Author:    "Test <test@example.com> 1700000000 +0000",
		Committer: "Test <test@example.com> 1700000000 +0000",
		ParentIDs: parents,
		TreeID:    treeOID,
		Message:   msg,
	})
	if err != nil {
		t.Fatalf("write commit: %v", err)
	}
	return oid
}

// updateRef points the given ref at oid through the git CLI.
func updateRef(t *testing.T, repoPath, ref string, oid []byte) {
	t.Helper()
	cmd := exec.Command("git", "--git-dir", repoPath, "update-ref", ref, hex.EncodeToString(oid))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("update-ref %s: %v\n%s", ref, err, out)
	}
}

// listTreePaths returns every path reachable from a commit's tree via `git ls-tree -r`.
func listTreePaths(t *testing.T, repoPath, oid string) []string {
	t.Helper()
	cmd := exec.Command("git", "--git-dir", repoPath, "ls-tree", "-r", "--name-only", oid)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ls-tree: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// resolveRef returns the hex commit oid that ref currently points at.
func resolveRef(t *testing.T, repoPath, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "--git-dir", repoPath, "rev-parse", ref)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// TestReplayerDrop is an end-to-end check that Drop strips the targeted path
// from history and rewrites the branch ref to point at the new commit.
func TestReplayerDrop(t *testing.T) {
	requireGit(t)
	repo, odb := makeBareRepo(t)

	// Tree layout:
	//   keep.txt  (blob)
	//   vendor/   (subtree)
	//     dep.txt (blob)
	keepBlob := writeBlob(t, odb, "keep me\n")
	depBlob := writeBlob(t, odb, "vendor stuff\n")

	subTree, err := odb.WriteTree(&gitobj.Tree{Entries: []*gitobj.TreeEntry{
		{Name: "dep.txt", Filemode: 0100644, Oid: depBlob},
	}})
	if err != nil {
		t.Fatalf("write subtree: %v", err)
	}
	rootTree, err := odb.WriteTree(&gitobj.Tree{Entries: []*gitobj.TreeEntry{
		{Name: "keep.txt", Filemode: 0100644, Oid: keepBlob},
		{Name: "vendor", Filemode: 040000, Oid: subTree},
	}})
	if err != nil {
		t.Fatalf("write root tree: %v", err)
	}
	c1 := writeCommit(t, odb, rootTree, nil, "initial\n")

	// Second commit adds another file inside vendor/, exercising multi-generation rewrite.
	depBlob2 := writeBlob(t, odb, "another vendor file\n")
	subTree2, err := odb.WriteTree(&gitobj.Tree{Entries: []*gitobj.TreeEntry{
		{Name: "dep.txt", Filemode: 0100644, Oid: depBlob},
		{Name: "more.txt", Filemode: 0100644, Oid: depBlob2},
	}})
	if err != nil {
		t.Fatalf("write subtree2: %v", err)
	}
	rootTree2, err := odb.WriteTree(&gitobj.Tree{Entries: []*gitobj.TreeEntry{
		{Name: "keep.txt", Filemode: 0100644, Oid: keepBlob},
		{Name: "vendor", Filemode: 040000, Oid: subTree2},
	}})
	if err != nil {
		t.Fatalf("write root tree2: %v", err)
	}
	c2 := writeCommit(t, odb, rootTree2, [][]byte{c1}, "add vendor file\n")

	updateRef(t, repo, "refs/heads/main", c2)

	// Replayer manages its own internal ODB; the outer one is closed via t.Cleanup.
	r, err := NewReplayer(context.Background(), repo, 1, false /*verbose*/)
	if err != nil {
		t.Fatalf("new replayer: %v", err)
	}
	defer r.Close() // nolint

	// confirm=true skips interactive prompts; prune=false skips git gc to keep the test fast.
	if err := r.Drop(NewMatcher([]string{"vendor"}), true, false); err != nil {
		t.Fatalf("drop: %v", err)
	}

	// The tree referenced by main must no longer contain any vendor path.
	newHead := resolveRef(t, repo, "refs/heads/main")
	if newHead == hex.EncodeToString(c2) {
		t.Fatalf("ref refs/heads/main should be rewritten, but stays at %s", newHead)
	}
	paths := listTreePaths(t, repo, newHead)
	for _, p := range paths {
		if strings.HasPrefix(p, "vendor/") || p == "vendor" {
			t.Fatalf("vendor path %q still present after Drop, paths=%v", p, paths)
		}
	}
	// keep.txt must survive the rewrite.
	if !slices.Contains(paths, "keep.txt") {
		t.Fatalf("keep.txt missing from rewritten tree, paths=%v", paths)
	}

	// History length stays at 2 since Drop rewrites trees but never drops empty commits.
	cmd := exec.Command("git", "--git-dir", repo, "rev-list", "--count", newHead)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-list count: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "2" {
		t.Fatalf("expect 2 commits after drop, got %s", got)
	}
}
