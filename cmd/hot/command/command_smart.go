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
	"github.com/charmbracelet/huh"
)

type Smart struct {
	Paths    []string `arg:"" name:"path" help:"Path to repositories" default:"." type:"path"`
	Limit    int64    `short:"L" name:"limit" optional:"" help:"Large file limit size, supported units: KB,MB,GB,K,M,G" default:"20m" type:"size"`
	Confirm  bool     `short:"Y" name:"confirm" help:"Confirm rewriting local branches and tags"`
	Prune    bool     `short:"P" name:"prune" help:"Prune repository when commits are rewritten"`
	FullPath bool     `short:"F" name:"full-path" help:"Show full path"`
	ALL      bool     `short:"A" name:"all" help:"Remove all large blobs"`
}

func (c *Smart) Run(g *Globals) error {
	for _, p := range c.Paths {
		if err := c.doOnce(g, p); err != nil {
			return err
		}
	}
	return nil
}

func multiSelect(i int, totalBatches int, input []string) ([]string, error) {
	var paths []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("%s [%s - %d/%d]:", tr.W("Which files need to be deleted"), tr.W("Batch"), i+1, totalBatches)).
				Options(huh.NewOptions(input...)...).
				Value(&paths)))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return paths, nil
}

func newMatcher(sz *stat.SizeExecutor, matchAll bool) replay.Matcher {
	if matchAll {
		return sz
	}
	larges := sz.Paths()
	selected := make([]string, 0, len(larges))
	totalBatches := (len(larges) + 19) / 20
	for i := range totalBatches {
		pathsLen := len(larges)
		if pathsLen == 0 {
			break
		}
		minGroup := min(20, pathsLen)
		var paths []string
		var err error
		paths, err = multiSelect(i, totalBatches, larges[0:minGroup])
		if err != nil {
			fmt.Fprintf(os.Stderr, "multi select error: %v\n", err)
			return nil
		}
		larges = larges[minGroup:]
		selected = append(selected, paths...)
	}
	if len(selected) == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr, "%s %d\n", tr.W("The total number of files that will be deleted is:"), len(selected))
	return replay.NewEqualer(selected)
}

func (c *Smart) doOnce(g *Globals, p string) error {
	repoPath := git.RevParseRepoPath(context.Background(), p)
	trace.DbgPrint("check %s size ...", p)
	e := stat.NewSizeExecutor(c.Limit, c.FullPath)
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
	defer r.Close() // nolint
	if err := r.Drop(matcher, c.Confirm, c.Prune); err != nil {
		fmt.Fprintf(os.Stderr, "rewrite repo error: %v\n", err)
		return err
	}
	return nil
}
