package zeta

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta/config"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

func TestWorktree(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/xh3",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	cc, err := r.odb.Commit(t.Context(), plumbing.NewHash("0942fdefc71cd54066e99b56dd47570ae2f18f41eb2406d65b0092e9c9d2efaf"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo commit error: %v\n", err)
		return
	}
	tt, err := r.odb.Tree(t.Context(), cc.Tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo tree error: %v\n", err)
		return
	}
	changes, err := w.diffTreeWithStaging(t.Context(), tt, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo tree error: %v\n", err)
		return
	}
	for _, c := range changes {
		fmt.Fprintf(os.Stderr, "%v\n", c.String())
	}
}

func TestWorktree2(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/k4",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	cc, err := r.odb.Commit(t.Context(), plumbing.NewHash("a8b63b8ba5256d03587ab2c595b5b3f0473c1b7c5498f022d9b36cf1139e0a21"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo commit error: %v\n", err)
		return
	}
	tt, err := r.odb.Tree(t.Context(), cc.Tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo tree error: %v\n", err)
		return
	}
	changes, err := w.diffTreeWithStaging(t.Context(), tt, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo tree error: %v\n", err)
		return
	}
	for _, c := range changes {
		fmt.Fprintf(os.Stderr, "%v\n", c.String())
	}
}

func TestWorktree3(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/xh7",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	changes, err := w.Status(t.Context(), true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkout error: %v\n", err)
	}
	for name, c := range changes {
		fmt.Fprintf(os.Stderr, "%s %c\n", name, c.Worktree)
	}
}

func TestCheckout(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/xh3",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	cc, err := r.odb.Commit(t.Context(), plumbing.NewHash("0942fdefc71cd54066e99b56dd47570ae2f18f41eb2406d65b0092e9c9d2efaf"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo commit error: %v\n", err)
		return
	}
	if err := w.Checkout(t.Context(), &CheckoutOptions{Hash: cc.Hash}); err != nil {
		fmt.Fprintf(os.Stderr, "checkout error: %v\n", err)
	}
}

func TestStatus(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/bb",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	changes, err := w.Status(t.Context(), true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status error: %v\n", err)
	}
	for name, c := range changes {
		fmt.Fprintf(os.Stderr, "%s %c\n", name, c.Worktree)
	}
}

func TestStatus2(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/k3",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	changes, err := w.Status(t.Context(), true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status error: %v\n", err)
	}
	for name, c := range changes {
		fmt.Fprintf(os.Stderr, "%s %c\n", name, c.Worktree)
	}
}

func TestIndex(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/k3",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	tree, err := w.odb.Tree(t.Context(), plumbing.NewHash("e23e0364b4c49bbfd179ce65bb76a224aa8a3a27dea25e691bed31ed8b7a693b"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open tree error: %v\n", err)
		return
	}
	err = w.resetIndex(t.Context(), tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reset index error: %v\n", err)
	}
}

func TestCommit(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/xh4",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	oid, err := w.Commit(t.Context(), &CommitOptions{All: true, Message: []string{"new commit message"}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkout error: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "new oid: %s\n", oid)
}

func TestCommit2(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/xh5",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	oid, err := w.Commit(t.Context(), &CommitOptions{All: true, Message: []string{"new commit message ------>\n"}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkout error: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "new oid: %s\n", oid)
}

func WalkNode(ctx context.Context, n noder.Noder) {
	nodes, err := n.Children(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk error: %s\n", err)
		return
	}
	for _, a := range nodes {
		if a.IsDir() {
			WalkNode(ctx, a)
			continue
		}
		fmt.Fprintf(os.Stderr, "%s\n", a.String())
	}
}

func TestTreeNode(t *testing.T) {
	o, err := odb.NewODB("/tmp/xh5/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open odb error: %s\n", err)
		return
	}
	defer o.Close()
	tree, err := o.Tree(t.Context(), plumbing.NewHash("dee3c85319b94c91616e16014cdf2839ca7d0d3cf8412a633ac7169440fc1a58"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open tree error: %s\n", err)
		return
	}
	node := object.NewTreeRootNode(tree, noder.NewSparseTreeMatcher([]string{"dir3", "dir1"}), true)
	WalkNode(t.Context(), node)
}

func TestCalculateChunk(t *testing.T) {
	chunks := calculateChunk(strengthen.GiByte*10+strengthen.MiByte, strengthen.GiByte)
	fmt.Fprintf(os.Stderr, "size: %d\n", strengthen.GiByte*10+strengthen.MiByte)
	for i, c := range chunks {
		fmt.Fprintf(os.Stderr, "%d: offset: %d size: %s\n", i, c.offset, strengthen.FormatSize(c.size))
	}
	chunks = calculateChunk(strengthen.GiByte*1+strengthen.MiByte, config.FragmentSize)
	fmt.Fprintf(os.Stderr, "size: %d\n", strengthen.GiByte*1+strengthen.MiByte)
	for i, c := range chunks {
		fmt.Fprintf(os.Stderr, "%d: offset: %d size: %s\n", i, c.offset, strengthen.FormatSize(c.size))
	}
	chunks = calculateChunk(3221000000, config.FragmentSize)
	fmt.Fprintf(os.Stderr, "size: %d\n", strengthen.GiByte*1+strengthen.MiByte)
	for i, c := range chunks {
		fmt.Fprintf(os.Stderr, "%d: offset: %d size: %s\n", i, c.offset, strengthen.FormatSize(c.size))
	}
}

func TestCalculateChunk2(t *testing.T) {
	chunks := calculateChunk(strengthen.GiByte*10-strengthen.MiByte, strengthen.GiByte)
	fmt.Fprintf(os.Stderr, "size: %d\n", strengthen.GiByte*10+strengthen.MiByte)
	for i, c := range chunks {
		fmt.Fprintf(os.Stderr, "%d: offset: %d size: %s\n", i, c.offset, strengthen.FormatSize(c.size))
	}
	chunks = calculateChunk(strengthen.GiByte*1, config.FragmentSize)
	fmt.Fprintf(os.Stderr, "size: %d\n", strengthen.GiByte*1+strengthen.MiByte)
	for i, c := range chunks {
		fmt.Fprintf(os.Stderr, "%d: offset: %d size: %s\n", i, c.offset, strengthen.FormatSize(c.size))
	}
}

func TestMask(t *testing.T) {
	mode := filemode.Regular
	mode |= filemode.Executable
	fmt.Fprintf(os.Stderr, "%o\n", mode)
	mode = mode&^filemode.Executable | filemode.Regular
	fmt.Fprintf(os.Stderr, "%o\n", mode)
	mode = filemode.Regular | filemode.Fragments
	mode |= filemode.Executable
	fmt.Fprintf(os.Stderr, "%o\n", mode)
	mode = mode&^filemode.Executable | filemode.Regular
	fmt.Fprintf(os.Stderr, "%o\n", mode)
}

func TestGrep(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/xh5",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	w := r.Worktree()
	result, err := w.Grep(t.Context(), &GrepOptions{
		Patterns: []*regexp.Regexp{regexp.MustCompile("import")},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "grep error: %v\n", err)
		return
	}
	for _, a := range result {
		fmt.Fprintf(os.Stderr, "%v\n", a.String())
	}
}

func TestStat(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/xh5",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	cc, err := r.odb.Commit(t.Context(), plumbing.NewHash("cc9bc711ee644d0441d5d0a63bba5548d4bb3e06ee99edc0e27aa0c57d57efe8"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open commit error: %v\n", err)
		return
	}
	ss, err := cc.StatsContext(t.Context(), noder.NewSparseTreeMatcher(r.Core.SparseDirs), &object.PatchOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "stats commit error: %v\n", err)
		return
	}
	for _, s := range ss {
		fmt.Fprintf(os.Stderr, "%s\n", s.String())
	}
}

func TestResolveImmutableEntries(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/k4",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	w := r.Worktree()
	h := &treeBuilder{
		w:               w,
		trees:           make(map[string]*object.Tree),
		readOnlyEntries: make(map[string]*object.TreeEntry),
	}
	oid, err := r.Revision(t.Context(), "HEAD^")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve error: %v\n", err)
		return
	}
	cc, err := r.odb.Commit(t.Context(), oid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve commit error: %v\n", err)
		return
	}
	tree, err := r.odb.Tree(t.Context(), cc.Tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve tree error: %v\n", err)
		return
	}
	if err := h.resolveReadOnlyEntries(t.Context(), tree, "", noder.NewSparseTreeMatcher(r.Core.SparseDirs)); err != nil {
		fmt.Fprintf(os.Stderr, "resolve error: %v\n", err)
		return
	}
	entries := make([]*object.TreeEntry, 0, 100)
	for k, e := range h.readOnlyEntries {
		entries = append(entries, &object.TreeEntry{Name: k, Hash: e.Hash})

	}
	sort.Sort(object.SubtreeOrder(entries))
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "%s %s\n", e.Hash, e.Name)
	}
}

func TestMatcher(t *testing.T) {
	m := NewMatcher([]string{"**/*.java"})
	ss := []string{"**/*.java", "test.java"}
	for _, s := range ss {
		fmt.Fprintf(os.Stderr, "%s %v\n", s, m.Match(s))
	}
}

func TestMatcher2(t *testing.T) {
	m := NewMatcher([]string{"sigma/appops/"})
	ss := []string{"sigma/appops/intelligent_engine/stability_service/debugbase/pre/stack.yaml", "sigma/appops/intelligent_engine/stability_service/debugbase/prod/ci-test/settings.yaml"}
	for _, s := range ss {
		fmt.Fprintf(os.Stderr, "%s %v\n", s, m.Match(s))
	}
}

func checkTreeSize(ctx context.Context, o *odb.ODB, tree *object.Tree, parent string, action string) error {
	entries := make([]*object.TreeEntry, 0, len(tree.Entries))
	entries = append(entries, tree.Entries...)
	sort.Sort(object.SubtreeOrder(entries))
	if !tree.Equal(&object.Tree{
		Entries: entries,
	}) {
		fmt.Fprintf(os.Stderr, "%s not order\n", tree.Hash)
		for i := 0; i < len(entries); i++ {
			fmt.Fprintf(os.Stderr, "%s|%s\n", tree.Entries[i].Name, entries[i].Name)
		}
	}
	for _, e := range tree.Entries {
		if e.Type() != object.TreeObject {
			continue
		}
		name := path.Join(parent, e.Name)
		if e.Size != 0 {
			fmt.Fprintf(os.Stderr, "[%s] tree size not zero: %s\n", action, name)
		}
		sub, err := o.Tree(ctx, e.Hash)
		if plumbing.IsNoSuchObject(err) {
			continue
		}
		if err != nil {
			return err
		}
		if err := checkTreeSize(ctx, o, sub, name, action); err != nil {
			return err
		}
	}
	return nil
}

func TestCat4(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/xh7",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	t0, err := r.odb.Tree(t.Context(), plumbing.NewHash("2dfb5cfe652f747551d9c03da557af00b147e103ae6810e8e226662dc9b05a9c"))
	if err != nil {
		return
	}
	_ = checkTreeSize(t.Context(), r.odb, t0, "", "oldtree")
	t1, err := r.odb.Tree(t.Context(), plumbing.NewHash("bb126e78f3b5ce90fc53602b1c6180999893d4cefb995e11bbb5e09ca5f026ad"))
	if err != nil {
		return
	}
	_ = checkTreeSize(t.Context(), r.odb, t0, "", "newtree")
	if t0.Equal(t1) {
		fmt.Fprintf(os.Stderr, "equal %s %s\n", t0.Hash, t1.Hash)
	}
}

func TestMergeBase(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/b3",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	c1, err := r.odb.Commit(t.Context(), plumbing.NewHash("3d9fb9964feffd6da7a46552e6f3c1a5360c106de2f7de13642b3bfce6970d95"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve commit error: %v\n", err)
		return
	}
	c2, err := r.odb.Commit(t.Context(), plumbing.NewHash("16f7d9dcac2ec114f63e4468c08dad952adeb05ae9ca59ea9e9b0ad1cd6a730d"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve commit error: %v\n", err)
		return
	}
	bases, err := c1.MergeBase(t.Context(), c2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check merge-base error: %v\n", err)
		return
	}
	if len(bases) == 0 {
		fmt.Fprintf(os.Stderr, "merge: refusing to merge unrelated histories\n")
		return
	}
	for _, c := range bases {
		_ = c.Pretty(os.Stderr)
	}
}

func TestMergeBase2(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/b2",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	c1, err := r.odb.Commit(t.Context(), plumbing.NewHash("abf364f16c0def448adb4db318d6677523a8b09d5947e502bb9e0d32e9c4b7b6"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve commit error: %v\n", err)
		return
	}
	c2, err := r.odb.Commit(t.Context(), plumbing.NewHash("1a3738abb6463fd98fcfea561942e9ed8d515137b901119c3e6f1d0c0bda4663"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve commit error: %v\n", err)
		return
	}
	b, err := c1.IsAncestor(t.Context(), c2)
	fmt.Fprintf(os.Stderr, "c1 isAncestor c2: %v %v\n", b, err)
	b, err = c2.IsAncestor(t.Context(), c1)
	fmt.Fprintf(os.Stderr, "c2 isAncestor c1: %v %v\n", b, err)
	bases, err := c1.MergeBase(t.Context(), c2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check merge-base error: %v\n", err)
		return
	}
	if len(bases) == 0 {
		fmt.Fprintf(os.Stderr, "merge: refusing to merge unrelated histories\n")
		return
	}
	for _, c := range bases {
		_ = c.Pretty(os.Stderr)
	}
}

func TestLsTreeFilter(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/k6",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	tree, err := r.resolveTree(t.Context(), "HEAD:")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve tree %s\n", err)
		return
	}
	entries, err := r.lsTreeRecurseFilter(t.Context(), tree, NewMatcher([]string{"*.k", "sigma/appops/intelligent_engine/business_intelligence-recommendation_engine/tapeargo"}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls tree %s\n", err)
		return
	}
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "%s\n", e.Path)
	}
}

func TestLsTreeFilter2(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/zeta-extra",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	tree, err := r.resolveTree(t.Context(), "HEAD:")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve tree %s\n", err)
		return
	}
	entries, err := r.lsTreeRecurseFilter(t.Context(), tree, NewMatcher([]string{"cmd", "*.c"}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls tree %s\n", err)
		return
	}
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "%s\n", e.Path)
	}
}

type A struct {
	B string `toml:"b"`
	C string `toml:"c"`
}

func TestEncode(t *testing.T) {
	a := &A{
		B: `'{"appname":"tcloudantcodeweb","name":"tcloudantcodewebTBaseCache","type":"G","zdcUrl":"http://127.0.0.1"}'`,
		C: `"'{\"appname\":\"tcloudantcodeweb\",\"name\":\"tcloudantcodewebTBaseCache\",\"type\":\"G\",\"zdcUrl\":\"AAAAAAA\"}'"`,
	}
	_ = toml.NewEncoder(os.Stderr).Encode(a)
}

func TestMode(t *testing.T) {
	fmt.Fprintf(os.Stderr, "%o\n", filemode.Regular&filemode.Executable)
}
