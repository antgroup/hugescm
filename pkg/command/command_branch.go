// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type Branch struct {
	ShowCurrent bool     `name:"show-current" help:"Show current branch name"`
	List        bool     `name:"list" short:"l" help:"List branches. With optional <pattern>..."`
	Copy        bool     `name:"copy" short:"c" help:"Copy a branch and its reflog"`
	ForceCopy   bool     `short:"C" shortonly:"" help:"Copy a branch, even if target exists"`
	Delete      bool     `name:"delete" short:"d" help:"Delete fully merged branch"`
	ForceDelete bool     `short:"D" shortonly:"" help:"Delete branch (even if not merged)"`
	Move        bool     `name:"move" short:"m" help:"Move/rename a branch and its reflog"`
	ForceMove   bool     `short:"M" shortonly:"" help:"Move/rename a branch, even if target exists"`
	Force       bool     `name:"force" short:"f" help:"Force creation, move/rename, deletion"`
	Args        []string `arg:"" optional:"" name:"args" help:""`
}

const (
	branchSummaryFormat = `%szeta branch [<options>] [-f] <branchname> [<start-point>]
%szeta branch [<options>] [-l] [<pattern>...]
%szeta branch [<options>] (-d | -D) <branchname>...
%szeta branch [<options>] (-m | -M) [<old-branch>] <new-branch>
%szeta branch [<options>] (-c | -C) [<old-branch>] <new-branch>
%szeta branch --show-current`
)

func (b *Branch) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(branchSummaryFormat, W("Usage: "), or, or, or, or, or)
}

func (b *Branch) IsMove() bool {
	return b.ForceMove || b.Move
}

func (b *Branch) IsDelete() bool {
	return b.ForceDelete || b.Delete
}

func (b *Branch) IsForceMove() bool {
	return b.ForceMove || b.Force
}

func (b *Branch) IsForceDelete() bool {
	return b.ForceDelete || b.Force
}

func (b *Branch) IsForceCopy() bool {
	return b.ForceCopy || b.Force
}

func (b *Branch) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	if b.ShowCurrent {
		return r.ShowCurrent(os.Stdout)
	}
	if b.List {
		return r.ListBranch(context.Background(), b.Args)
	}
	if b.IsMove() {
		if len(b.Args) < 2 {
			diev("branch name required, eg: zeta branch --move <from> <to>")
			return ErrArgRequired
		}
		return r.MoveBranch(b.Args[0], b.Args[1], b.IsForceMove())
	}
	if b.IsDelete() {
		if len(b.Args) < 1 {
			diev("branch name required, eg: zeta branch --delete <branchname>")
			return ErrArgRequired
		}
		return r.RemoveBranch(b.Args, b.IsForceDelete())
	}
	if len(b.Args) == 0 {
		return r.ListBranch(context.Background(), nil)
	}
	from := "HEAD"
	if len(b.Args) >= 2 {
		from = b.Args[1]
	}
	return r.CreateBranch(context.Background(), b.Args[0], from, b.IsForceCopy(), false)
}
