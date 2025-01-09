// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type Diff struct {
	NoIndex         bool     `name:"no-index" help:"Compares two given paths on the filesystem"`
	NameOnly        bool     `name:"name-only" help:"Show only names of changed files"`
	NameStatus      bool     `name:"name-status" help:"Show names and status of changed files"`
	Numstat         bool     `name:"numstat" help:"Show numeric diffstat instead of patch"`
	Stat            bool     `name:"stat" help:"Show diffstat instead of patch"`
	Shortstat       bool     `name:"shortstat" help:"Output only the last line of --stat format"`
	Z               bool     `short:"z" shortonly:"" help:"Output diff-raw with lines terminated with NUL"`
	Staged          bool     `name:"staged" help:"Compare the differences between the staging area and <revision>"`
	Cached          bool     `name:"cached" help:"Compare the differences between the staging area and <revision>"`
	Textconv        bool     `name:"textconv" help:"Converting text to Unicode"`
	MergeBase       string   `name:"merge-base" help:"If --merge-base is given, use the common ancestor of <commit> and HEAD instead" placeholder:"<merge-base>"`
	Histogram       bool     `name:"histogram" help:"Generate a diff using the \"Histogram diff\" algorithm"`
	ONP             bool     `name:"onp" help:"Generate a diff using the \"O(NP) diff\" algorithm"`
	Myers           bool     `name:"myers" help:"Generate a diff using the \"Myers diff\" algorithm"`
	Patience        bool     `name:"patience" help:"Generate a diff using the \"Patience diff\" algorithm"`
	Minimal         bool     `name:"minimal" help:"Spend extra time to make sure the smallest possible diff is produced"`
	DiffAlgorithm   string   `name:"diff-algorithm" help:"Choose a diff algorithm, supported: histogram|onp|myers|patience|minimal" placeholder:"<algorithm>"`
	Output          string   `name:"output" help:"Output to a specific file instead of stdout" placeholder:"<file>"`
	From            string   `arg:"" optional:"" name:"from" help:""`
	To              string   `arg:"" optional:"" name:"to" help:""`
	passthroughArgs []string `kong:"-"`
}

const (
	diffSummaryFormat = `%s zeta diff [<options>] [<commit>] [--] [<path>...]
%s zeta diff [<options>] --cached [<commit>] [--] [<path>...]
%s zeta diff [<options>] <commit> <commit> [--] [<path>...]
%s zeta diff [<options>] <commit>...<commit> [--] [<path>...]
%s zeta diff [<options>] <blob> <blob>
%s zeta diff [<options>] --no-index [--] <path> <path>`
)

func (c *Diff) Summary() string {
	or := W("   or: ")
	return fmt.Sprintf(diffSummaryFormat, W("Usage: "), or, or, or, or, or)
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
		return diferenco.AlgorithmFromName(c.DiffAlgorithm)
	}
	switch {
	case c.Histogram:
		return diferenco.Histogram, nil
	case c.ONP:
		return diferenco.ONP, nil
	case c.Myers:
		return diferenco.Myers, nil
	case c.Patience:
		return diferenco.Patience, nil
	case c.Minimal:
		return diferenco.Minimal, nil
	default:
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
		Numstat:    c.Numstat,
		Stat:       c.Stat,
		Shortstat:  c.Shortstat,
		NewLine:    c.NewLine(),
		NewOutput:  c.NewOutput,
		PathSpec:   slashPaths(c.passthroughArgs),
		From:       c.From,
		To:         c.To,
		Staged:     c.Staged || c.Cached,
		MergeBase:  c.MergeBase,
		Textconv:   c.Textconv,
		Algorithm:  a,
	}
	if len(c.To) == 0 {
		if from, to, ok := strings.Cut(c.From, "..."); ok {
			opts.From = from
			opts.To = to
			opts.Way3 = true
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

func (c *Diff) NewOutput(ctx context.Context) (zeta.Printer, error) {
	if len(c.Output) != 0 {
		if err := os.MkdirAll(filepath.Dir(c.Output), 0755); err != nil {
			return nil, err
		}
		fd, err := os.Create(c.Output)
		if err != nil {
			return nil, err
		}
		return &zeta.WrapPrinter{WriteCloser: fd}, nil
	}
	return zeta.NewPrinter(ctx), nil
}

func (c *Diff) render(u *diferenco.Unified) error {
	opts := &zeta.DiffOptions{
		NameOnly:   c.NameOnly,
		NameStatus: c.NameStatus,
		Numstat:    c.Numstat,
		Stat:       c.Stat,
		Shortstat:  c.Shortstat,
		NewLine:    c.NewLine(),
		NewOutput:  c.NewOutput,
		NoRename:   true,
	}
	switch {
	case c.Numstat, c.Stat, c.Shortstat:
		s := u.Stat()
		name := c.From
		if c.From != c.To {
			name = object.PathRenameCombine(c.From, c.To)
		}
		return opts.ShowStats(context.Background(), object.FileStats{
			object.FileStat{
				Name:     name,
				Addition: s.Addition,
				Deletion: s.Deletion,
			},
		})
	default:
		return opts.ShowPatch(context.Background(), []*diferenco.Unified{u})
	}
}

func (c *Diff) nameStatus() error {
	w, err := c.NewOutput(context.Background())
	if err != nil {
		return err
	}
	defer w.Close()
	if c.NameOnly {
		fmt.Fprintf(w, "%s%c", c.From, c.NewLine())
		return nil
	}
	fmt.Fprintf(w, "%c    %s%c", 'M', c.To, c.NewLine())
	return nil
}

func (c *Diff) diffNoIndex(g *Globals) error {
	if len(c.From) == 0 || len(c.To) == 0 {
		die("missing arg, example: zeta diff --no-index from to")
		return ErrArgRequired
	}
	c.From = cleanPath(c.From)
	c.To = cleanPath(c.To)
	if c.NameOnly || c.NameStatus {
		return c.nameStatus()
	}

	a, err := c.checkAlgorithm()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeta diff --no-index: parse options error: %v\n", err)
		return err
	}
	g.DbgPrint("from %s to %s", c.From, c.To)
	from, err := zeta.ReadContent(c.From, c.Textconv)
	if err != nil {
		diev("zeta diff --no-index hash error: %v", err)
		return err
	}
	to, err := zeta.ReadContent(c.To, c.Textconv)
	if err != nil && err != diferenco.ErrNonTextContent {
		diev("zeta diff --no-index read text error: %v", err)
		return err
	}
	if from.IsBinary || to.IsBinary {
		return c.render(&diferenco.Unified{
			From:     &diferenco.File{Name: c.From, Hash: from.Hash, Mode: uint32(from.Mode)},
			To:       &diferenco.File{Name: c.To, Hash: to.Hash, Mode: uint32(to.Mode)},
			IsBinary: true,
		})
	}
	if from.Hash == to.Hash {
		return c.render(&diferenco.Unified{
			From:     &diferenco.File{Name: c.From, Hash: from.Hash, Mode: uint32(from.Mode)},
			To:       &diferenco.File{Name: c.To, Hash: to.Hash, Mode: uint32(to.Mode)},
			IsBinary: false,
		})
	}
	u, err := diferenco.DoUnified(context.Background(), &diferenco.Options{
		From: &diferenco.File{Name: c.From, Hash: from.Hash, Mode: uint32(from.Mode)},
		To:   &diferenco.File{Name: c.To, Hash: to.Hash, Mode: uint32(to.Mode)},
		S1:   from.Text,
		S2:   to.Text,
		A:    a,
	})
	if err != nil {
		diev("zeta diff --no-index error: %v", err)
		return err
	}
	return c.render(u)
}

func (c *Diff) Run(g *Globals) error {
	if c.NoIndex {
		return c.diffNoIndex(g)
	}
	if _, _, err := zeta.FindZetaDir(g.CWD); err != nil {
		return c.diffNoIndex(g)
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
