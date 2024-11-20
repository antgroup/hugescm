package filesystem

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
)

func WalkNode(n noder.Noder) {
	nodes, err := n.Children(context.TODO())
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk error: %s\n", err)
		return
	}
	for _, a := range nodes {
		if a.IsDir() {
			WalkNode(a)
			continue
		}
		fmt.Fprintf(os.Stderr, "%s\n", a.String())
	}
}

func TestNode(t *testing.T) {
	n := NewRootNode("/tmp/fsnode", noder.NewSparseTreeMatcher([]string{"a", "a/a", "c"}))
	WalkNode(n)
}
func TestNode2(t *testing.T) {
	n := NewRootNode("/tmp/fsnode", noder.NewSparseTreeMatcher([]string{}))
	WalkNode(n)
}

func TestNode3(t *testing.T) {
	n := NewRootNode("/tmp/xh5", noder.NewSparseTreeMatcher([]string{"dir1", "dir3"}))
	WalkNode(n)
}
