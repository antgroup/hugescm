// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/antgroup/hugescm/modules/diferenco/color"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/pkg/zeta"
)

// Blame shows what revision and author last modified each line of a file.
type Blame struct {
	Args []string `arg:"" optional:"" name:"args" help:"[<revision>] <path>"`
}

const (
	blameSummaryFormat = `%szeta blame [<options>] [<revision>] <path>`
	blameDateFormat    = "2006-01-02 15:04:05 -0700"
)

func (c *Blame) Summary() string {
	return fmt.Sprintf(blameSummaryFormat, W("Usage: "))
}

func (c *Blame) Run(ctx context.Context, g *Globals) error {
	var rev, path string
	switch len(c.Args) {
	case 0:
		die(c.Summary())
		return ErrArgRequired
	case 1:
		path = c.Args[0]
	case 2:
		rev, path = c.Args[0], c.Args[1]
	default:
		die("too many arguments, example: zeta blame [<revision>] <path>")
		return ErrArgRequired
	}
	r, err := zeta.Open(ctx, &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint

	result, err := r.BlameFile(ctx, rev, path)
	if err != nil {
		diev("zeta blame error: %v", err)
		return err
	}
	w := zeta.NewPrinter(ctx)
	defer w.Close() // nolint
	_, err = io.WriteString(w, renderBlame(result, w.ColorMode()))
	return err
}

// renderBlame formats a BlameResult in git-blame style:
//
//	<hash> (<author> <date> <line-number>) <text>
//
// With a color level above LevelNone the commit hash is yellow (matching the
// diff theme's commit color) and the date and line number are dimmed; the
// plain output is byte-identical to BlameResult.String().
func renderBlame(b *zeta.BlameResult, level term.Level) string {
	var buf strings.Builder
	if len(b.Lines) == 0 {
		return ""
	}
	// max line number length
	mlnl := len(strconv.Itoa(len(b.Lines)))
	// max author length
	mal := 0
	for _, l := range b.Lines {
		mal = max(mal, utf8.RuneCountInString(l.AuthorName))
	}
	plainFormat := fmt.Sprintf("%%s (%%-%ds %%s %%%dd) %%s\n", mal, mlnl)
	coloredFormat := fmt.Sprintf("%%s (%%-%ds %%s) %%s\n", mal)

	colored := level != term.LevelNone
	for ln, l := range b.Lines {
		date := l.Date.Format(blameDateFormat)
		if !colored {
			_, _ = fmt.Fprintf(&buf, plainFormat, l.Hash.String()[:8],
				l.AuthorName, date, ln+1, l.Text)
			continue
		}
		num := fmt.Sprintf("%*d", mlnl, ln+1)
		_, _ = fmt.Fprintf(&buf, coloredFormat,
			color.Yellow+l.Hash.String()[:8]+color.Reset,
			l.AuthorName,
			color.Faint+date+" "+num+color.Reset,
			l.Text)
	}
	return buf.String()
}
