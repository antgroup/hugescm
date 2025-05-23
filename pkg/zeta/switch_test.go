package zeta

import (
	"fmt"
	"os"
	"testing"
)

func TestSwitch(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/tmp/xh4",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "switch error: %v\n", err)
		return
	}
	defer r.Close() // nolint
	if err := r.SwitchBranch(t.Context(), "dev-4", &SwitchOptions{}); err != nil {
		fmt.Fprintf(os.Stderr, "switch error: %v\n", err)
		return
	}
}

func TestCat(t *testing.T) {
	r, err := Open(t.Context(), &OpenOptions{
		Worktree: "/tmp/blat",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "switch error: %v\n", err)
		return
	}
	defer r.Close() // nolint
	_ = r.Cat(t.Context(), &CatOptions{Object: "2be5d4418893425e546a6146fbda18eac95ea9a7fbb05faab02096738a974a11"})
}
