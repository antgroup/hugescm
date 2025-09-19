// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"context"

	"github.com/antgroup/hugescm/cmd/hot/pkg/stat"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/trace"
)

type Az struct {
	Paths    []string `arg:"" name:"path" help:"Path to repositories" default:"." type:"path"`
	Limit    int64    `short:"L" name:"limit" optional:"" help:"Large file limit size, supported units: KB,MB,GB,K,M,G" default:"10m" type:"size"`
	FullPath bool     `short:"F" name:"full-path" help:"Show full path"`
}

func (c *Az) Run(g *Globals) error {
	for _, p := range c.Paths {
		if err := c.azOnce(p); err != nil {
			return err
		}
	}
	return nil
}

// git cat-file --batch-check --batch-all-objects
func (c *Az) azOnce(p string) error {
	repoPath := git.RevParseRepoPath(context.Background(), p)
	trace.DbgPrint("begin analysis repository: %v large file: %v", repoPath, strengthen.FormatSize(c.Limit))
	return stat.Az(context.Background(), repoPath, c.Limit, c.FullPath)
}
