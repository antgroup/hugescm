// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Remove struct {
	DryRun   bool     `name:"dry-run" short:"n" help:"Dry run"`
	Quiet    bool     `name:"quiet" short:"q" help:"Do not list removed files"`
	Cached   bool     `name:"cached" help:"Only remove from the index"`
	Force    bool     `name:"force" short:"f" help:"Override the up-to-date check"`
	Recurse  bool     `short:"r" shortonly:"" help:"Allow recursive removal"`
	PathSpec []string `arg:"" optional:"" name:"pathspec" help:"Path specification, similar to Git path matching mode"`
}

func (c *Remove) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
		Quiet:    c.Quiet,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	w := r.Worktree()
	if err := w.Remove(context.Background(), c.PathSpec, &zeta.RemoveOptions{
		Recurse: c.Recurse,
		Cached:  c.Cached,
		Force:   c.Force,
		DryRun:  c.DryRun}); err != nil {
		diev("zeta rm: %s", err)
		return err
	}
	return nil
}
