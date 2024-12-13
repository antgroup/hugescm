// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package diferenco

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DefaultContextLines is the number of unchanged lines of surrounding
// context displayed by Unified. Use ToUnified to specify a different value.
const DefaultContextLines = 3

type File struct {
	Path string
	Hash string
	Mode uint32
}

// unified represents a set of edits as a unified diff.
type Unified struct {
	// From is the name of the original file.
	From *File
	// To is the name of the modified file.
	To *File
	// IsBinary returns true if this patch is representing a binary file.
	IsBinary bool
	// Fragments returns true if this patch is representing a fragments file.
	IsFragments bool
	// Message prefix, eg: warning: something
	Message string
	// Hunks is the set of edit Hunks needed to transform the file content.
	Hunks []*Hunk
}

// String converts a unified diff to the standard textual form for that diff.
// The output of this function can be passed to tools like patch.
func (u Unified) String() string {
	if len(u.Hunks) == 0 {
		return ""
	}
	b := new(strings.Builder)
	if u.From != nil {
		fmt.Fprintf(b, "--- %s\n", u.From.Path)
	} else {
		fmt.Fprintf(b, "--- /dev/null\n")
	}
	if u.To != nil {
		fmt.Fprintf(b, "+++ %s\n", u.To.Path)
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
	FromLine int
	// The line in the original source where the hunk finishes.
	ToLine int
	// The set of line based edits to apply.
	Lines []Line
}

type Line struct {
	Kind    Operation
	Content string
}

type Options struct {
	From, To *File
	A, B     string
	Algo     Algorithm
}

func DoUnified(ctx context.Context, opts *Options) (*Unified, error) {
	sk := &Sink{
		Index: make(map[string]int),
	}
	a := sk.ParseLines(opts.A)
	b := sk.ParseLines(opts.B)
	var err error
	var changes []Change
	switch opts.Algo {
	case Histogram:
		if changes, err = HistogramDiff(ctx, a, b); err != nil {
			return nil, err
		}
	case Myers:
		if changes, err = MyersDiff(ctx, a, b); err != nil {
			return nil, err
		}
	case ONP:
		if changes, err = OnpDiff(ctx, a, b); err != nil {
			return nil, err
		}
	case Patience:
		if changes, err = PatienceDiff(ctx, a, b); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported algorithm")
	}
	return sk.ToUnified(opts.From, opts.To, changes, a, b, DefaultContextLines), nil
}
