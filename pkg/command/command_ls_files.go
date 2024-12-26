// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/antgroup/hugescm/pkg/zeta"
)

type LsFiles struct {
	Cached   bool     `name:"cached" short:"c" help:"Show cached files in the output (default)"`
	Deleted  bool     `name:"deleted" short:"d" help:"Show deleted files in the output"`
	Modified bool     `name:"modified" short:"m" help:"Show modified files in the output"`
	Others   bool     `name:"others" short:"o" help:"Show other files in the output"`
	Stage    bool     `name:"stage" short:"s" help:"Show staged contents' object name in the output"`
	Z        bool     `short:"z" shortonly:"" help:"Terminate entries with NUL byte"`
	JSON     bool     `name:"json" short:"j" help:"Data will be returned in JSON format"`
	Paths    []string `arg:"" name:"path" optional:"" help:"Given paths, show as match patterns; else, use root as sole argument"`
}

func (c *LsFiles) Run(g *Globals) error {
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
	opts := &zeta.LsFilesOptions{
		Z:     c.Z,
		JSON:  c.JSON,
		Paths: slashPaths(c.Paths),
	}
	switch {
	case c.Stage:
		opts.Mode = zeta.ListFilesStage
	case c.Deleted:
		opts.Mode = zeta.ListFilesDeleted
	case c.Modified:
		opts.Mode = zeta.ListFilesModified
	case c.Others:
		opts.Mode = zeta.ListFilesOthers
	}
	if err := w.LsFiles(context.Background(), opts); err != nil {
		diev("zeta ls-files error: %v", err)
		return err
	}
	return nil
}
