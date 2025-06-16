// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Rebase struct {
	Args     []string `arg:"" help:"Upstream and branch to rebase (upstream branch to compare against and branch to rebase)"`
	Onto     string   `name:"onto" help:"Rebase onto given branch" placeholder:"<revision>"`
	Abort    bool     `name:"abort" help:"Abort and checkout the original branch"`
	Continue bool     `name:"continue" help:"Continue"`
}

func (c *Rebase) Run(g *Globals) error {
	if c.Abort && c.Continue {
		diev("--abort is not compatible with --continue")
		return ErrFlagsIncompatible
	}
	if !(c.Abort || c.Continue) && len(c.Args) == 0 {
		fmt.Fprintf(os.Stderr, "Please specify which branch you want to rebase against.\n")
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

	opts := &zeta.RebaseOptions{
		Branch:   "HEAD",
		Onto:     c.Onto,
		Abort:    c.Abort,
		Continue: c.Continue,
	}
	if len(c.Args) > 0 {
		opts.Upstream = c.Args[0]
	}
	if len(c.Args) > 1 {
		opts.Branch = c.Args[1]
	}
	if err := w.Rebase(context.Background(), opts); err != nil {
		return err
	}
	return nil
}
