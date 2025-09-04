package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// Apply the changes introduced by some existing commit
type CherryPick struct {
	Revision string `arg:"" optional:"" name:"revision" help:"Existing commit" placeholder:"<revision>"`
	Abort    bool   `name:"abort" help:"Abort and checkout the original branch"`
	Continue bool   `name:"continue" help:"Continue"`
}

func (c *CherryPick) Run(g *Globals) error {
	if c.Abort && c.Continue {
		diev("--abort is not compatible with --continue")
		return ErrFlagsIncompatible
	}
	if !c.Abort && !c.Continue && len(c.Revision) == 0 {
		die("missing revision arg")
		return ErrArgRequired
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	w := r.Worktree()
	if err := w.CherryPick(context.Background(), &zeta.CherryPickOptions{
		From:     c.Revision,
		Abort:    c.Abort,
		Continue: c.Continue,
	}); err != nil {
		return err
	}
	return nil
}
