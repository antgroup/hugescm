package index

import (
	"testing"

	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

// makeIndex builds a tiny *index.Index with several siblings at the
// root so DiffTreeContext actually exercises frame sort/drop on the
// children slice.
func makeIndex() *index.Index {
	return &index.Index{Entries: []*index.Entry{
		{Name: "a.txt", Mode: filemode.Regular, Stage: index.Merged},
		{Name: "b.txt", Mode: filemode.Regular, Stage: index.Merged},
		{Name: "c.txt", Mode: filemode.Regular, Stage: index.Merged},
		{Name: "d.txt", Mode: filemode.Regular, Stage: index.Merged},
		{Name: "e.txt", Mode: filemode.Regular, Stage: index.Merged},
		{Name: "f.txt", Mode: filemode.Regular, Stage: index.Merged},
	}}
}

func equal(a, b noder.Hasher) bool { return false }

// TestRootNodeIsConsumedDestructively documents the invariant behind
// pkg/zeta/worktree_status.go: a mindex root must be consumed by at
// most one DiffTreeContext. The iterator mutates the per-node children
// slice in place (frame.Drop nils out the underlying array), so a
// second pass would observe nil entries and panic in frame.byName.
//
// This is a sentinel test: if the underlying behaviour ever changes
// (e.g. frame.New is taught to defensively copy), this test will
// start failing and we can relax the rebuild-per-diff constraint in
// status().
func TestRootNodeIsConsumedDestructively(t *testing.T) {
	idx := makeIndex()
	root := NewRootNode(t.Context(), idx, nil).(*Node)

	// Snapshot pre-state so we can prove the slice was mutated.
	preLen := len(root.children)
	if preLen == 0 {
		t.Fatalf("root should have children")
	}
	for i, c := range root.children {
		if c == nil {
			t.Fatalf("pre-state: children[%d] already nil", i)
		}
	}

	// First diff: any "from" is fine; we only care about the
	// iteration of root. nil "from" yields an Insert per entry but
	// also drives the iterator across every child of root.
	if _, err := merkletrie.DiffTreeContext(t.Context(), nil, root, equal); err != nil {
		t.Fatalf("first DiffTreeContext: %v", err)
	}

	// After consumption the children slice still has the same length
	// (Drop only narrows the iterator's view) but the underlying
	// array has been nil-ed out. This is what made reusing the root
	// panic in frame.byName.Less.
	if len(root.children) != preLen {
		t.Fatalf("len(children) changed: pre=%d post=%d", preLen, len(root.children))
	}
	var nilFound bool
	for _, c := range root.children {
		if c == nil {
			nilFound = true
			break
		}
	}
	if !nilFound {
		t.Fatalf("expected nil entries in root.children after a DiffTreeContext pass, got none")
	}
}

// TestRootNodeReuseAcrossDiffsPanics is the actual crash repro that
// motivated this guard. If anyone reintroduces the "share one
// indexRoot across both diffs" optimisation in status(), this test
// will catch it.
func TestRootNodeReuseAcrossDiffsPanics(t *testing.T) {
	idx := makeIndex()
	root := NewRootNode(t.Context(), idx, nil)

	if _, err := merkletrie.DiffTreeContext(t.Context(), nil, root, equal); err != nil {
		t.Fatalf("first DiffTreeContext: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected second DiffTreeContext over the same root to panic; it did not")
		}
	}()
	_, _ = merkletrie.DiffTreeContext(t.Context(), nil, root, equal)
}

// TestFreshRootPerDiffIsSafe shows the recipe used by status(): keep
// the *index.Index but rebuild the mindex root for each
// DiffTreeContext. This must complete without panicking.
func TestFreshRootPerDiffIsSafe(t *testing.T) {
	idx := makeIndex()
	ctx := t.Context()

	if _, err := merkletrie.DiffTreeContext(ctx, nil, NewRootNode(ctx, idx, nil), equal); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if _, err := merkletrie.DiffTreeContext(ctx, nil, NewRootNode(ctx, idx, nil), equal); err != nil {
		t.Fatalf("second pass: %v", err)
	}
}
