// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/pkg/stat"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

type Size struct {
	Paths    []string `arg:"" name:"path" help:"Path to repositories" default:"." type:"path"`
	Limit    int64    `short:"L" name:"limit" optional:"" help:"Large file limit size, supported units: KB,MB,GB,K,M,G" default:"20m" type:"size"`
	Extract  bool     `short:"E" name:"extract" optional:"" help:"Whether large files exist in the default branch"`
	FullPath bool     `short:"F" name:"full-path" help:"Show full path"`
}

func (c *Size) Run(g *Globals) error {
	for _, p := range c.Paths {
		if err := c.sizeOnce(p); err != nil {
			fmt.Fprintf(os.Stderr, "show repo '%s' size error: %v\n", p, err)
			return err
		}
	}
	return nil
}

func (c *Size) sizeOnce(p string) error {
	repoPath := git.RevParseRepoPath(context.Background(), p)
	trace.DbgPrint("check %s size ...", repoPath)
	e := stat.NewSizeExecutor(c.Limit, c.FullPath)
	if err := e.Run(context.Background(), repoPath, c.Extract); err != nil {
		return err
	}
	return nil
}
