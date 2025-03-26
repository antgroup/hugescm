package zeta

import (
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestLog(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/tmp/b3",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "switch error: %v\n", err)
		return
	}
	defer r.Close()

	commits, err := r.revList(t.Context(),
		plumbing.NewHash("6c4abb63943a42c4374e42b805113f2288b832319ccce79c12c836e59c41ccce"),
		[]plumbing.Hash{
			plumbing.NewHash("c0869060ede3e208c464cac81fd78e6f31cecb572a3450b9a7dce4784c6dab5f"),
		}, nil)
	if err != nil {
		die_error("log range base error: %v", err)
		return
	}
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		fmt.Fprintf(os.Stderr, "%s: %s\n", c.Hash, c.Subject())
	}

}
