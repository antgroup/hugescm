// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package mc

import (
	"context"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH; skip integration test")
	}
}

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

// TestMigratorSHA1ToSHA256 is an end-to-end check that migrating a sha1
// repository produces a sha256 repository where:
//   - all three source commits are migrated;
//   - the ref topology depth matches;
//   - each migrated commit's message matches the original;
//   - keep.txt / extra.txt remain present in the migrated root tree.
func TestMigratorSHA1ToSHA256(t *testing.T) {
	requireGit(t)

	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.git")
	dst := filepath.Join(tmp, "dst.git")
	ctx := context.Background()

	if err := git.NewRepo(ctx, src, "main", true, git.HashSHA1); err != nil {
		t.Fatalf("init src: %v", err)
	}
	odb, err := git.NewODB(src, git.HashSHA1)
	if err != nil {
		t.Fatalf("open src odb: %v", err)
	}

	// Three-generation history: c1 -> c2 -> c3.
	blob := writeBlob(t, odb, "hello\n")
	tree, err := odb.WriteTree(&gitobj.Tree{Entries: []*gitobj.TreeEntry{
		{Name: "keep.txt", Filemode: 0100644, Oid: blob},
	}})
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	c1 := writeCommit(t, odb, tree, nil, "c1\n")

	blob2 := writeBlob(t, odb, "hello again\n")
	tree2, err := odb.WriteTree(&gitobj.Tree{Entries: []*gitobj.TreeEntry{
		{Name: "keep.txt", Filemode: 0100644, Oid: blob2},
	}})
	if err != nil {
		t.Fatalf("write tree2: %v", err)
	}
	c2 := writeCommit(t, odb, tree2, [][]byte{c1}, "c2\n")

	blob3 := writeBlob(t, odb, "third\n")
	tree3, err := odb.WriteTree(&gitobj.Tree{Entries: []*gitobj.TreeEntry{
		{Name: "keep.txt", Filemode: 0100644, Oid: blob2},
		{Name: "extra.txt", Filemode: 0100644, Oid: blob3},
	}})
	if err != nil {
		t.Fatalf("write tree3: %v", err)
	}
	c3 := writeCommit(t, odb, tree3, [][]byte{c2}, "c3\n")

	if out, err := exec.Command("git", "--git-dir", src, "update-ref", "refs/heads/main", hex.EncodeToString(c3)).CombinedOutput(); err != nil {
		_ = odb.Close()
		t.Fatalf("update-ref: %v\n%s", err, out)
	}
	if err := odb.Close(); err != nil {
		t.Fatalf("close src odb: %v", err)
	}

	// Run the migration.
	mig, err := NewMigrator(ctx, &MigrateOptions{
		From:    src,
		To:      dst,
		Format:  "sha256",
		Bare:    true,
		Verbose: false,
		StepEnd: 4,
	})
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	defer mig.Close() // nolint

	if err := mig.Execute(ctx); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Destination repo must report sha256 as its object format.
	if got := strings.TrimSpace(runGit(t, dst, "rev-parse", "--show-object-format")); got != "sha256" {
		t.Fatalf("dst format = %s, want sha256", got)
	}

	// Total commit count must remain at 3.
	count := strings.TrimSpace(runGit(t, dst, "rev-list", "--count", "refs/heads/main"))
	if count != "3" {
		t.Fatalf("dst rev-list count = %s, want 3", count)
	}

	// Walk back from HEAD: HEAD -> c3, HEAD~1 -> c2, HEAD~2 -> c1.
	for i, want := range []string{"c3", "c2", "c1"} {
		ref := "refs/heads/main"
		if i > 0 {
			ref = ref + "~" + strconv.Itoa(i)
		}
		got := strings.TrimSpace(runGit(t, dst, "log", "-1", "--format=%s", ref))
		if got != want {
			t.Fatalf("commit at %s message = %q, want %q", ref, got, want)
		}
	}

	// Both keep.txt and extra.txt must survive in the migrated root tree.
	paths := strings.Split(strings.TrimSpace(runGit(t, dst, "ls-tree", "-r", "--name-only", "refs/heads/main")), "\n")
	mset := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		mset[p] = struct{}{}
	}
	for _, want := range []string{"keep.txt", "extra.txt"} {
		if _, ok := mset[want]; !ok {
			t.Fatalf("dst ls-tree missing %q, paths=%v", want, paths)
		}
	}
}

func runGit(t *testing.T, repoPath string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"--git-dir", repoPath}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
