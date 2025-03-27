// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// --since=<date>, --after=<date>
// Show commits more recent than a specific date.

// --since-as-filter=<date>
// Show all commits more recent than a specific date. This visits all commits in the range, rather than stopping at the first commit which is older than a specific date.

// --until=<date>, --before=<date>
// Show commits older than a specific date.

// --author=<pattern>, --committer=<pattern>
// Limit the commits output to ones with author/committer header lines that match the specified pattern (regular expression). With more than one --author=<pattern>, commits whose author matches any of
// the given patterns are chosen (similarly for multiple --committer=<pattern>).

type Log struct {
	Revision      string   `arg:"" optional:"" name:"revision-range" help:"Revision range"`
	OrderByDate   bool     `name:"date-order" help:"Order by committer date"`
	OrderByAuthor bool     `name:"author-date-order" help:"Order by author date"`
	Reverse       bool     `name:"reverse" help:"Reverse order"`
	JSON          bool     `name:"json" short:"j" help:"Data will be returned in JSON format"`
	paths         []string `kong:"-"`
}

const (
	logSummaryFormat = `%szeta log [<options>] [<revision-range>] [[--] <path>...]`
)

func (c *Log) Summary() string {
	return fmt.Sprintf(logSummaryFormat, W("Usage: "))
}

func (c *Log) Passthrough(paths []string) {
	c.paths = append(c.paths, paths...)
}

func (c *Log) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	opts := &zeta.LogCommandOptions{
		Revision:             c.Revision,
		Order:                zeta.LogOrderTopo, // --topo-order
		OrderByCommitterDate: c.OrderByDate,
		OrderByAuthorDate:    c.OrderByAuthor,
		Paths:                slashPaths(c.paths),
		Reverse:              c.Reverse,
		FormatJSON:           c.JSON,
	}
	if err := r.Log(context.Background(), opts); err != nil {
		return err
	}
	return nil
}
