package refs

import (
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestBackend(t *testing.T) {
	repoPath := "/tmp/repo/zeta.zeta"
	_ = os.MkdirAll("/tmp/repo/zeta.zeta", 0755)
	b := NewBackend(repoPath)
	refs := []string{
		"refs/heads/mainline",
		"refs/heads/dev",
		"refs/tags/v1.0.0",
		"refs/remotes/origin/master",
	}
	for _, r := range refs {
		err := b.Update(plumbing.NewHashReference(plumbing.ReferenceName(r), plumbing.NewHash("adba50d9794b9ef3f7ec8cbc680f7f1fa3fbf9df0ac8d1f9b9ccab6d941bc11b")), nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	if err := b.Packed(); err != nil {
		fmt.Fprintf(os.Stderr, "packed refs error: %v\n", err)
		return
	}
	_ = b.Update(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/dev"), plumbing.NewHash("d84149926219c5a85da48051f2b3ad296f3ade3c5cb91dac4848d84de28c12dd")), nil)
}

func TestRemove(t *testing.T) {
	repoPath := "/tmp/repo/zeta.zeta"
	b := NewBackend(repoPath)
	_ = b.ReferenceRemove(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/dev"), plumbing.NewHash("d84149926219c5a85da48051f2b3ad296f3ade3c5cb91dac4848d84de28c12dd")))
}
