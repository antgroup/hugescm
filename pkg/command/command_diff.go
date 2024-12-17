// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type Diff struct {
	NameOnly        bool     `name:"name-only" help:"Show only names of changed files"`
	NameStatus      bool     `name:"name-status" help:"Show names and status of changed files"`
	Numstat         bool     `name:"numstat" help:"Show numeric diffstat instead of patch"`
	Stat            bool     `name:"stat" help:"Show diffstat instead of patch"`
	ShortStat       bool     `name:"shortstat" help:"Output only the last line of --stat format"`
	Z               bool     `name:":z" short:"z" help:"Output diff-raw with lines terminated with NUL"`
	Staged          bool     `name:"staged" help:"Compare the differences between the staging area and <revision>"`
	Cached          bool     `name:"cached" help:"Compare the differences between the staging area and <revision>"`
	Textconv        bool     `name:"textconv" help:"Convert text to Unicode and compare differences"`
	MergeBase       string   `name:"merge-base" help:"If --merge-base is given, use the common ancestor of <commit> and HEAD instead"`
	Output          string   `name:"output" help:"Output to a specific file instead of stdout" placeholder:"<file>"`
	Histogram       bool     `name:"histogram" help:"Generate a diff using the \"Histogram diff\" algorithm"`
	ONP             bool     `name:"onp" help:"Generate a diff using the \"O(NP) diff\" algorithm"`
	Myers           bool     `name:"myers" help:"Generate a diff using the \"Myers diff\" algorithm"`
	Patience        bool     `name:"patience" help:"Generate a diff using the \"Patience diff\" algorithm"`
	Minimal         bool     `name:"minimal" help:"Spend extra time to make sure the smallest possible diff is produced"`
	DiffAlgorithm   string   `name:"diff-algorithm" help:"Choose a diff algorithm, supported: histogram|onp|myers|patience|minimal"`
	From            string   `arg:"" optional:"" name:"from" help:"Revision from"`
	To              string   `arg:"" optional:"" name:"to" help:"Revision to"`
	passthroughArgs []string `kong:"-"`
}

const (
	diffSummaryFormat = `%s zeta diff [<options>] [<commit>] [--] [<path>...]
%s zeta diff [<options>] --cached [<commit>] [--] [<path>...]
%s zeta diff [<options>] <commit> <commit> [--] [<path>...]
%s zeta diff [<options>] <commit>...<commit> [--] [<path>...]`
)

func (c *Diff) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(diffSummaryFormat, W("Usage: "), or, or, or)
}

func (c *Diff) NewLine() byte {
	if c.Z {
		return '\x00'
	}
	return '\n'
}

func (c *Diff) Passthrough(paths []string) {
	c.passthroughArgs = append(c.passthroughArgs, paths...)
}

func (c *Diff) checkAlgorithm() (diferenco.Algorithm, error) {
	if len(c.DiffAlgorithm) != 0 {
		return diferenco.ParseAlgorithm(c.DiffAlgorithm)
	}
	if c.Histogram {
		return diferenco.Histogram, nil
	}
	if c.ONP {
		return diferenco.ONP, nil
	}
	if c.Myers {
		return diferenco.Myers, nil
	}
	if c.Patience {
		return diferenco.Patience, nil
	}
	if c.Minimal {
		return diferenco.Minimal, nil
	}
	return diferenco.Unspecified, nil
}

func (c *Diff) NewOptions() (*zeta.DiffOptions, error) {
	a, err := c.checkAlgorithm()
	if err != nil {
		return nil, err
	}
	opts := &zeta.DiffOptions{
		NameOnly:   c.NameOnly,
		NameStatus: c.NameStatus,
		NumStat:    c.Numstat,
		Stat:       c.Stat,
		ShortStat:  c.ShortStat,
		Staged:     c.Staged || c.Cached,
		NewLine:    c.NewLine(),
		PathSpec:   slashPaths(c.passthroughArgs),
		From:       c.From,
		To:         c.To,
		MergeBase:  c.MergeBase,
		Textconv:   c.Textconv,
		Algorithm:  a,
		NewOutput:  c.NewOutput,
	}
	if len(c.To) == 0 {
		if from, to, ok := strings.Cut(c.From, "..."); ok {
			opts.From = from
			opts.To = to
			opts.W3 = true
			return opts, nil
		}
		if from, to, ok := strings.Cut(c.From, ".."); ok {
			opts.From = from
			opts.To = to
			return opts, nil
		}
	}
	return opts, nil
}

func (c *Diff) NewOutput(ctx context.Context) (io.WriteCloser, bool, error) {
	if len(c.Output) != 0 {
		if err := os.MkdirAll(filepath.Dir(c.Output), 0755); err != nil {
			return nil, false, err
		}
		fd, err := os.Create(c.Output)
		return fd, false, err
	}
	printer := zeta.NewPrinter(ctx)
	return printer, printer.UseColor(), nil
}

func (c *Diff) Run(g *Globals) error {
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
	opts, err := c.NewOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse options error: %v\n", err)
		return err
	}
	if err = w.DiffContext(context.Background(), opts); err != nil {
		return err
	}
	return nil
}
