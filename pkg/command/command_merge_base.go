// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// usage: zeta merge-base [-a | --all] <commit> <commit>...
//    or: zeta merge-base [-a | --all] --octopus <commit>...
//    or: zeta merge-base --is-ancestor <commit> <commit>

const (
	mergeBaseSummaryFormat = `%szeta merge-base [-a | --all] <commit> <commit>...
%szeta merge-base --is-ancestor <commit> <commit>`
)

type MergeBase struct {
	// --is-ancestor
	All        bool     `name:"all" short:"a" negatable:"" default:"false" help:"Output all common ancestors"`
	IsAncestor bool     `name:"is-ancestor" help:"Is the first one ancestor of the other?"`
	Args       []string `arg:"" name:"commit"`
}

func (c *MergeBase) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(mergeBaseSummaryFormat, W("Usage: "), or)
}

func (c *MergeBase) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	if c.IsAncestor {
		if len(c.Args) != 2 {
			diev("Need tow revision,  eg: zeta merge-base --is-ancestor A B")
			return ErrArgRequired
		}
		return r.IsAncestor(context.Background(), c.Args[0], c.Args[1])
	}
	if len(c.Args) < 2 {
		diev("At least two versions are required,  eg: zeta merge-base A B")
		return ErrArgRequired
	}
	return r.MergeBase(context.Background(), c.Args, c.All)
}
