package zeta

import (
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestBlame(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/private/tmp/hugescm-dev",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo error: %v\n", err)
		return
	}
	defer r.Close()
	cc, err := r.odb.Commit(t.Context(), plumbing.NewHash("4b2982c5c8835dfc3c1a8d0eddca9100e1aee1b7e7b9da44160bc9de99aa0b77"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open commit error: %v\n", err)
		return
	}
	b, err := Blame(t.Context(), cc, "pkg/zeta/worktree_diff.go")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open commit error: %v\n", err)
		return
	}
	for _, line := range b.Lines {
		fmt.Fprintf(os.Stderr, "%s %s %s\n", line.Author, line.Date, line.Text)
	}
}
