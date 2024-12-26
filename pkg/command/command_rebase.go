// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Rebase struct {
	Onto     string `name:"onto" help:"Rebase onto given branch" placeholder:"<revision>"`
	Abort    bool   `name:"abort" help:"Abort and checkout the original branch"`
	Continue bool   `name:"continue" help:"Continue"`
}

func (c *Rebase) Run(g *Globals) error {
	if c.Abort && c.Continue {
		diev("--abort is not compatible with --continue")
		return ErrFlagsIncompatible
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	w := r.Worktree()
	if err := w.Rebase(context.Background(), &zeta.RebaseOptions{
		Onto:     c.Onto,
		Abort:    c.Abort,
		Continue: c.Continue,
	}); err != nil {
		return err
	}
	return nil
}
