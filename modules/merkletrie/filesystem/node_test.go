package filesystem

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
)

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

func TestNode(t *testing.T) {
	n := NewRootNode("/tmp/fsnode", noder.NewSparseTreeMatcher([]string{"a", "a/a", "c"}))
	WalkNode(t.Context(), n)
}
func TestNode2(t *testing.T) {
	n := NewRootNode("/tmp/fsnode", noder.NewSparseTreeMatcher([]string{}))
	WalkNode(t.Context(), n)
}

func TestNode3(t *testing.T) {
	n := NewRootNode("/tmp/xh5", noder.NewSparseTreeMatcher([]string{"dir1", "dir3"}))
	WalkNode(t.Context(), n)
}
