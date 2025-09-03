// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/pkg/replay"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

type Remove struct {
	CWD      string   `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Paths    []string `arg:"" name:"Paths" help:"Path to remove in repository, support wildcards" type:"string"`
	Confirm  bool     `short:"Y" name:"confirm" help:"Confirm rewriting local branches and tags"`
	Prune    bool     `short:"P" name:"prune" help:"Prune repository when commits are rewritten"`
	Graft    bool     `short:"G" name:"graft" help:"Grafting mode"`
	HeadOnly bool     `short:"H" name:"head-only" help:"Graft only the default branch"`
}

func (c *Remove) Run(g *Globals) error {
	repoPath := git.RevParseRepoPath(context.Background(), c.CWD)
	trace.DbgPrint("repository location: %v", repoPath)
	matcher := replay.NewMatcher(c.Paths)
	if c.Graft {
		r, err := replay.NewReplayer(context.Background(), repoPath, 4, g.Verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "new replayer error: %v\n", err)
			return err
		}
		defer r.Close() // nolint
		if err := r.Graft(matcher, c.Confirm, c.Prune, c.HeadOnly); err != nil {
			fmt.Fprintf(os.Stderr, "graft repo error: %v\n", err)
			return err
		}
		return nil
	}
	r, err := replay.NewReplayer(context.Background(), repoPath, 3, g.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new replayer error: %v\n", err)
		return err
	}
	defer r.Close() // nolint
	if err := r.Drop(matcher, c.Confirm, c.Prune); err != nil {
		fmt.Fprintf(os.Stderr, "replay repo error: %v\n", err)
		return err
	}
	return nil
}
