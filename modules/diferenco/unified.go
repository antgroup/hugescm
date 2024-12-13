// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package diferenco

import (
	"context"
	"errors"
)

// DefaultContextLines is the number of unchanged lines of surrounding
// context displayed by Unified. Use ToUnified to specify a different value.
const DefaultContextLines = 3

type File struct {
	Path string
	Hash string
	Mode int
}

// unified represents a set of edits as a unified diff.
type Unified struct {
	// From is the name of the original file.
	From File
	// To is the name of the modified file.
	To File
	// IsBinary returns true if this patch is representing a binary file.
	IsBinary bool
	// Fragments returns true if this patch is representing a fragments file.
	IsFragments bool
	// Hunks is the set of edit Hunks needed to transform the file content.
	Hunks []*Hunk
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
	Old, New string
	From     File
	To       File
	ALGO     Algorithm
}

func DoUnified(ctx context.Context, opts *Options) (*Unified, error) {
	sk := &Sink{
		Index: make(map[string]int),
	}
	a := sk.ParseLines(opts.Old)
	b := sk.ParseLines(opts.New)
	var changes []Change
	switch opts.ALGO {
	case Histogram:
		changes = HistogramDiff(a, b)
	case Myers:
		changes = MyersDiff(a, b)
	case ONP:
		changes = OnpDiff(a, b)
	default:
		return nil, errors.New("unsupported algorithm")
	}
	return sk.ToUnified(opts.From, opts.To, changes, a, b, DefaultContextLines), nil
}
