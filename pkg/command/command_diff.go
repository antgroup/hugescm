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
	"github.com/antgroup/hugescm/modules/plumbing"
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
	Z               bool     `name:":z" short:"z" help:"Output diff-raw with lines terminated with NUL"`
	Staged          bool     `name:"staged" help:"Compare the differences between the staging area and <revision>"`
	Cached          bool     `name:"cached" help:"Compare the differences between the staging area and <revision>"`
	TextConv        bool     `name:"textconv" help:"Convert text to Unicode and compare differences"`
	MergeBase       string   `name:"merge-base" help:"If --merge-base is given, use the common ancestor of <commit> and HEAD instead"`
	Output          string   `name:"output" help:"Output to a specific file instead of stdout" placeholder:"<file>"`
	Histogram       bool     `name:"histogram" help:"Generate a diff using the \"Histogram diff\" algorithm"`
	ONP             bool     `name:"onp" help:"Generate a diff using the \"O(NP) diff\" algorithm"`
	Myers           bool     `name:"myers" help:"Generate a diff using the \"Myers diff\" algorithm"`
	Patience        bool     `name:"patience" help:"Generate a diff using the \"Patience diff\" algorithm"`
	Minimal         bool     `name:"minimal" help:"Spend extra time to make sure the smallest possible diff is produced"`
	DiffAlgorithm   string   `name:"diff-algorithm" help:"Choose a diff algorithm, supported: histogram|onp|myers|patience|minimal"`
	From            string   `arg:"" optional:"" name:"from" help:"From"`
	To              string   `arg:"" optional:"" name:"to" help:"To"`
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
		TextConv:   c.TextConv,
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
			name = fmt.Sprintf("%s => %s", c.From, c.To)
		}
		opts.ShowStats(context.Background(), object.FileStats{
			object.FileStat{
				Name:     name,
				Addition: s.Addition,
				Deletion: s.Deletion,
			},
		})
	default:
		opts.ShowPatch(context.Background(), []*diferenco.Unified{u})
	}
	return nil
}

func (c *Diff) nameStatus() error {
	w, _, err := c.NewOutput(context.Background())
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

func hashNoIndexFile(p string) (string, error) {
	fd, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer fd.Close()
	h := plumbing.NewHasher()
	if _, err := io.Copy(h, fd); err != nil {
		return "", err
	}
	return h.Sum().String(), nil
}

func (c *Diff) diffNoIndex(g *Globals) error {
	if len(c.From) == 0 || len(c.To) == 0 {
		die("missing arg, example: zeta diff --no-index from to")
		return ErrArgRequired
	}
	if c.NameOnly || c.NameStatus {
		return c.nameStatus()
	}

	a, err := c.checkAlgorithm()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeta diff --no-index: parse options error: %v\n", err)
		return err
	}
	g.DbgPrint("from %s to %s", c.From, c.To)
	var isBinary bool
	fromHash, err := hashNoIndexFile(c.From)
	if err != nil {
		diev("zeta diff --no-index hash error: %v", err)
		return err
	}
	from, err := readText(c.From, c.TextConv)
	if err != nil && err != diferenco.ErrNonTextContent {
		diev("zeta diff --no-index read text error: %v", err)
		return err
	}
	if err == diferenco.ErrNonTextContent {
		isBinary = true
	}
	toHash, err := hashNoIndexFile(c.To)
	if err != nil {
		diev("zeta diff --no-index hash error: %v", err)
		return err
	}
	if fromHash == toHash {
		return c.render(&diferenco.Unified{
			From:     &diferenco.File{Name: c.From, Hash: fromHash},
			To:       &diferenco.File{Name: c.To, Hash: toHash},
			IsBinary: isBinary,
		})
	}
	to, err := readText(c.To, c.TextConv)
	if err != nil && err != diferenco.ErrNonTextContent {
		diev("zeta diff --no-index read text error: %v", err)
		return err
	}

	if err == diferenco.ErrNonTextContent {
		isBinary = true
	}
	if isBinary {
		return c.render(&diferenco.Unified{
			From:     &diferenco.File{Name: c.From, Hash: fromHash},
			To:       &diferenco.File{Name: c.To, Hash: toHash},
			IsBinary: true,
		})
	}
	u, err := diferenco.DoUnified(context.Background(), &diferenco.Options{
		From: &diferenco.File{Name: c.From, Hash: fromHash},
		To:   &diferenco.File{Name: c.To, Hash: toHash},
		S1:   from,
		S2:   to,
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
