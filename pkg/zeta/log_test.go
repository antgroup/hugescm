package zeta

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestLog(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/tmp/blat-zeta",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "log error: %v\n", err)
		return
	}
	defer r.Close()

	commits, err := r.revList(t.Context(),
		plumbing.NewHash("dffa478d973aed6d6af1d9a32c3e07bc61fdc98eddc76f5f36aa69d004d3aad4"),
		[]plumbing.Hash{
			plumbing.NewHash("0efd923d06041c04de8034195821efdc02a26eb6633d7651d8df1b0e70362c65"),
		}, nil)
	if err != nil {
		die_error("log range base error: %v", err)
		return
	}
	slices.Reverse(commits)
	for _, c := range commits {
		fmt.Fprintf(os.Stderr, "%s: %s\n", c.Hash, c.Subject())
	}

}
