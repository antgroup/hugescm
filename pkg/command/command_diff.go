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
	From            string   `arg:"" optional:"" name:"from" help:"Revision from"`
	To              string   `arg:"" optional:"" name:"to" help:"Revision to"`
	passthroughArgs []string `kong:"-"`
	useColor        bool     `kong:"-"`
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

func (c *Diff) NewOptions() *zeta.DiffContextOptions {
	opts := &zeta.DiffContextOptions{
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
		UseColor:   c.useColor,
		Textconv:   c.Textconv,
	}
	if len(c.To) == 0 {
		if from, to, ok := strings.Cut(c.From, "..."); ok {
			opts.From = from
			opts.To = to
			opts.ThreeWayCompare = true
			return opts
		}
		if from, to, ok := strings.Cut(c.From, ".."); ok {
			opts.From = from
			opts.To = to
			return opts
		}
	}
	return opts
}

func (c *Diff) NewOut(ctx context.Context) (io.WriteCloser, error) {
	if len(c.Output) != 0 {
		if err := os.MkdirAll(filepath.Dir(c.Output), 0755); err != nil {
			return nil, err
		}
		fd, err := os.Create(c.Output)
		return fd, err
	}
	printer := zeta.NewPrinter(ctx)
	c.useColor = printer.UseColor()
	return printer, nil
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
	newCtx, cancelCtx := context.WithCancelCause(context.Background())
	defer cancelCtx(nil)
	out, err := c.NewOut(newCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new output file error: %v\n", err)
		return err
	}
	defer out.Close()
	if err = w.DiffContext(newCtx, c.NewOptions(), out); err != nil {
		cancelCtx(err)
	}
	return err
}
