// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package diferenco

import (
	"context"
	"fmt"
	"strings"
)

// DefaultContextLines is the number of unchanged lines of surrounding
// context displayed by Unified. Use ToUnified to specify a different value.
const DefaultContextLines = 3

type File struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
	Mode uint32 `json:"mode"`
}

// unified represents a set of edits as a unified diff.
type Unified struct {
	// From is the name of the original file.
	From *File `json:"from,omitempty"`
	// To is the name of the modified file.
	To *File `json:"to,omitempty"`
	// IsBinary returns true if this patch is representing a binary file.
	IsBinary bool `json:"binary"`
	// Fragments returns true if this patch is representing a fragments file.
	IsFragments bool `json:"fragments"`
	// Message prefix, eg: warning: something
	Message string `json:"message"`
	// Hunks is the set of edit Hunks needed to transform the file content.
	Hunks []*Hunk `json:"hunks,omitempty"`
}

func (u Unified) Stat() FileStat {
	s := FileStat{Hunks: len(u.Hunks)}
	for _, h := range u.Hunks {
		ins, del := h.Stat()
		s.Addition += ins
		s.Deletion += del
	}
	return s
}

// String converts a unified diff to the standard textual form for that diff.
// The output of this function can be passed to tools like patch.
func (u Unified) String() string {
	if len(u.Hunks) == 0 {
		return ""
	}
	b := new(strings.Builder)
	if u.From != nil {
		fmt.Fprintf(b, "--- %s\n", u.From.Name)
	} else {
		fmt.Fprintf(b, "--- /dev/null\n")
	}
	if u.To != nil {
		fmt.Fprintf(b, "+++ %s\n", u.To.Name)
	} else {
		fmt.Fprintf(b, "+++ /dev/null\n")
	}

	for _, hunk := range u.Hunks {
		fromCount, toCount := 0, 0
		for _, l := range hunk.Lines {
			switch l.Kind {
			case Delete:
				fromCount++
			case Insert:
				toCount++
			default:
				fromCount++
				toCount++
			}
		}
		fmt.Fprint(b, "@@")
		if fromCount > 1 {
			fmt.Fprintf(b, " -%d,%d", hunk.FromLine, fromCount)
		} else if hunk.FromLine == 1 && fromCount == 0 {
			// Match odd GNU diff -u behavior adding to empty file.
			fmt.Fprintf(b, " -0,0")
		} else {
			fmt.Fprintf(b, " -%d", hunk.FromLine)
		}
		if toCount > 1 {
			fmt.Fprintf(b, " +%d,%d", hunk.ToLine, toCount)
		} else if hunk.ToLine == 1 && toCount == 0 {
			// Match odd GNU diff -u behavior adding to empty file.
			fmt.Fprintf(b, " +0,0")
		} else {
			fmt.Fprintf(b, " +%d", hunk.ToLine)
		}
		fmt.Fprint(b, " @@\n")
		for _, l := range hunk.Lines {
			switch l.Kind {
			case Delete:
				fmt.Fprintf(b, "-%s", l.Content)
			case Insert:
				fmt.Fprintf(b, "+%s", l.Content)
			default:
				fmt.Fprintf(b, " %s", l.Content)
			}
			if !strings.HasSuffix(l.Content, "\n") {
				fmt.Fprintf(b, "\n\\ No newline at end of file\n")
			}
		}
	}
	return b.String()
}

// Hunk represents a contiguous set of line edits to apply.
type Hunk struct {
	// The line in the original source where the hunk starts.
	FromLine int `json:"from_line"`
	// The line in the original source where the hunk finishes.
	ToLine int `json:"to_line"`
	// The set of line based edits to apply.
	Lines []Line `json:"lines,omitempty"`
}

func (h Hunk) Stat() (int, int) {
	var ins, del int
	for _, l := range h.Lines {
		switch l.Kind {
		case Delete:
			del++
		case Insert:
			ins++
		}
	}
	return ins, del
}

type Line struct {
	Kind    Operation `json:"kind"`
	Content string    `json:"content"`
}

func DoUnified(ctx context.Context, opts *Options) (*Unified, error) {
	sink := &Sink{
		Index: make(map[string]int),
	}
	a, err := sink.parseLines(opts.R1, opts.S1)
	if err != nil {
		return nil, err
	}
	b, err := sink.parseLines(opts.R2, opts.S2)
	if err != nil {
		return nil, err
	}
	changes, err := diffInternal(ctx, a, b, opts.A)
	if err != nil {
		return nil, err
	}
	return sink.ToUnified(opts.From, opts.To, changes, a, b, DefaultContextLines), nil
}
