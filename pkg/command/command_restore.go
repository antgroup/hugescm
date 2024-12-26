// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// https://git-scm.com/docs/git-restore/zh_HANS-CN

// Restore working tree files

type Restore struct {
	Source   string   `name:"source" short:"s" help:"Which tree-ish to checkout from" placeholder:"<revision>"`
	Staged   bool     `name:"staged" short:"S" negatable:"" help:"Restore the index"`
	Worktree bool     `name:"worktree" short:"W" negatable:"" help:"Restore the working tree (default)"`
	Paths    []string `arg:"" optional:"" name:"pathspec" help:"Limits the paths affected by the operation"`
}

func (c *Restore) Help() string {
	return fmt.Sprintf(`%s
 -W, --worktree, -S, --staged
 %s`, W("SYNOPSIS"), W("Specify restore location. By default, restores working tree. Use --staged for index only, or both for both."))
}

func (c *Restore) Run(g *Globals) error {
	if len(c.Paths) == 0 {
		die("you must specify path(s) to restore")
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
	defer r.Close()
	w := r.Worktree()
	opts := &zeta.RestoreOptions{
		Source:   c.Source,
		Staged:   c.Staged,
		Worktree: c.Worktree,
		Paths:    slashPaths(c.Paths),
	}
	if !opts.Staged && !c.Worktree {
		c.Worktree = true
	}
	if err := w.Restore(context.Background(), opts); err != nil {
		return err
	}
	return nil
}
