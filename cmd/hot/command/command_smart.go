// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/pkg/replay"
	"github.com/antgroup/hugescm/cmd/hot/pkg/size"
	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/survey"
	"github.com/antgroup/hugescm/modules/trace"
)

type Smart struct {
	Paths   []string `arg:"" name:"path" help:"Path to repositories" default:"." type:"path"`
	Limit   int64    `short:"L" name:"limit" optional:"" help:"Large file limit size, supported units: KB,MB,GB,K,M,G" default:"20m" type:"size"`
	Confirm bool     `short:"Y" name:"confirm" help:"Confirm rewriting local branches and tags"`
	Prune   bool     `short:"P" name:"prune" help:"Prune repository when commits are rewritten"`
	ALL     bool     `short:"A" name:"all" help:"Remove all large blobs"`
}

func (c *Smart) Run(g *Globals) error {
	for _, p := range c.Paths {
		if err := c.doOnce(g, p); err != nil {
			return err
		}
	}
	return nil
}

func newMatcher(sz *size.Executor, all bool) replay.Matcher {
	if all {
		return sz
	}
	largePaths := sz.Paths()
	removedPaths := make([]string, 0, len(largePaths))
	for i := range 40 {
		pathsLen := len(largePaths)
		if pathsLen == 0 {
			break
		}
		minSelect := min(20, pathsLen)
		var paths []string
		prompt := &survey.MultiSelect{
			Message:  fmt.Sprintf("%s [%s - %d]:", tr.W("Which files need to be deleted"), tr.W("Batch"), i+1),
			Options:  largePaths[0:minSelect],
			PageSize: 20,
		}
		largePaths = largePaths[minSelect:]
		if err := survey.AskOne(prompt, &paths); err != nil {
			fmt.Fprintf(os.Stderr, "abort: %v\n", err)
			return nil
		}
		removedPaths = append(removedPaths, paths...)
	}
	if len(removedPaths) == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr, "%s %d\n", tr.W("The total number of files that will be deleted is:"), len(removedPaths))
	return replay.NewEqualer(removedPaths)
}

func (c *Smart) doOnce(g *Globals, p string) error {
	repoPath := git.RevParseRepoPath(context.Background(), p)
	trace.DbgPrint("check %s size ...", p)
	e := size.NewExecutor(c.Limit)
	if err := e.Run(context.Background(), repoPath, false); err != nil {
		fmt.Fprintf(os.Stderr, "analyze repo size error: %v\n", err)
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
	r, err := replay.NewReplayer(context.Background(), repoPath, 3, g.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new rewriter error: %v\n", err)
		return err
	}
	defer r.Close()
	if err := r.Drop(matcher, c.Confirm, c.Prune); err != nil {
		fmt.Fprintf(os.Stderr, "rewrite repo error: %v\n", err)
		return err
	}
	return nil
}
