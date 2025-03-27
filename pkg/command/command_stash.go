// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// https://git-scm.com/docs/git-stash

type Stash struct {
	Push  StashPush  `cmd:"push" help:"Stash local changes and revert to HEAD" default:"1"`
	List  StashList  `cmd:"list" help:"List the stash entries that you currently have"`
	Show  StashShow  `cmd:"show" help:"Displays the diff of changes in a stash entry against the commit where it was created"`
	Clear StashClear `cmd:"clear" help:"Remove all the stash entries"`
	Drop  StashDrop  `cmd:"drop" help:"Remove a single stash entry from the list of stash entries"`
	Pop   StashPop   `cmd:"pop" help:"Apply and remove one stash"`
	Apply StashApply `cmd:"apply" help:"Like pop, but do not remove the state from the stash list"`
}

type StashPush struct {
	U bool `name:"include-untracked" short:"u" help:"Stashed untracked files with push/save, then cleaned with zeta clean"`
}

func (c *StashPush) Run(g *Globals) error {
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
	return w.StashPush(context.Background(), &zeta.StashPushOptions{U: c.U})
}

type StashList struct {
}

func (c *StashList) Run(g *Globals) error {
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
	return w.StashList(context.Background())
}

type StashShow struct {
	IncludeUntracked bool   `name:"include-untracked"`
	Stash            string `arg:"" optional:"" name:"stash" help:"Stash index" default:"stash@{0}"`
}

func (c *StashShow) Run(g *Globals) error {
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
	return w.StashShow(context.Background(), c.Stash)
}

type StashClear struct {
}

func (c *StashClear) Run(g *Globals) error {
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
	return w.StashClear(context.Background())
}

type StashDrop struct {
	Stash string `arg:"" optional:"" name:"stash" help:"Stash index" default:"stash@{0}"`
}

func (c *StashDrop) Run(g *Globals) error {
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
	return w.StashDrop(context.Background(), c.Stash)
}

// stash@{1}
type StashPop struct {
	Index bool   `name:"index" negatable:"" help:"Attempt to recreate the index"`
	Stash string `arg:"" optional:"" name:"stash" help:"Stash index" default:"stash@{0}"`
}

func (c *StashPop) Run(g *Globals) error {
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
	return w.StashPop(context.Background(), c.Stash)
}

type StashApply struct {
	Stash string `arg:"" optional:"" name:"stash" help:"Stash index" default:"stash@{0}"`
}

func (c *StashApply) Run(g *Globals) error {
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
	return w.StashApply(context.Background(), c.Stash)
}
