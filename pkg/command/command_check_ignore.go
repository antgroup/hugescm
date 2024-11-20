// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/zeta"
)

//  Debug gitignore / exclude files
//  https://git-scm.com/docs/git-check-ignore

type CheckIgnore struct {
	Stdin bool     `name:"stdin" help:"Read file names from stdin"`
	Z     bool     `name:":z" short:"z" help:"Terminate input and output records by a NUL character"`
	JSON  bool     `name:"json" short:"j" help:"Data will be returned in JSON format"`
	Paths []string `arg:"" name:"pathname" optional:"" help:"Pathname given via the command-line"`
}

const (
	ciSummaryFormat = `%szeta check-ignore [<options>] <pathname>...
%szeta check-ignore [<options>] --stdin`
)

func (c *CheckIgnore) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(ciSummaryFormat, W("Usage: "), or)
}

func (c *CheckIgnore) Run(g *Globals) error {
	if c.Stdin {
		if len(c.Paths) > 0 {
			die("cannot specify pathnames with --stdin")
			return ErrFlagsIncompatible
		}
	} else {
		if c.Z {
			die("-z only makes sense with --stdin")
			return ErrFlagsIncompatible
		}
		if len(c.Paths) == 0 {
			die("no path specified")
			return ErrFlagsIncompatible
		}
	}
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
	return w.DoCheckIgnore(context.Background(), &zeta.CheckIgnoreOption{
		Paths: slashPaths(c.Paths),
		Stdin: c.Stdin,
		Z:     c.Z,
		JSON:  c.JSON,
	})
}
