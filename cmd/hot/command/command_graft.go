// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/pkg/replay"
	"github.com/antgroup/hugescm/cmd/hot/pkg/stat"
	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

type Graft struct {
	Paths    []string `arg:"" name:"path" help:"Path to repositories" default:"." type:"path"`
	Limit    int64    `short:"L" name:"limit" optional:"" help:"Large file limit size, supported units: KB,MB,GB,K,M,G" default:"20m" type:"size"`
	Confirm  bool     `short:"Y" name:"confirm" help:"Confirm rewriting local branches and tags"`
	Prune    bool     `short:"P" name:"prune" help:"Prune repository when commits are rewritten"`
	HeadOnly bool     `short:"H" name:"head-only" help:"Graft only the default branch"`
	FullPath bool     `short:"F" name:"full-path" help:"Show full path"`
	ALL      bool     `short:"A" name:"all" help:"Remove all large blobs"`
}

func (c *Graft) Run(g *Globals) error {
	for _, p := range c.Paths {
		if err := c.doOnce(g, p); err != nil {
			return err
		}
	}
	return nil
}

func (c *Graft) doOnce(g *Globals, p string) error {
	repoPath := git.RevParseRepoPath(context.Background(), p)
	trace.DbgPrint("check %s size ...", repoPath)
	e := stat.NewSizeExecutor(c.Limit, c.FullPath)
	if err := e.Run(context.Background(), repoPath, false); err != nil {
		fmt.Fprintf(os.Stderr, "check repo size error: %v\n", err)
		return err
	}
	if len(e.Paths()) == 0 {
		return nil
	}
	if len(e.Paths()) > 300 {
		fmt.Fprintf(os.Stderr, "%s %d\n", tr.W("You can increase the file size limit, the number of large files: "), len(e.Paths()))
		return nil
	}
	matcher := newMatcher(e, c.ALL)
	if matcher == nil {
		return nil
	}
	r, err := replay.NewReplayer(context.Background(), repoPath, 4, g.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new replayer error: %v\n", err)
		return err
	}
	defer r.Close() // nolint
	if err := r.Graft(matcher, c.Confirm, c.Prune, c.HeadOnly); err != nil {
		fmt.Fprintf(os.Stderr, "replay repo error: %v\n", err)
		return err
	}
	return nil
}
