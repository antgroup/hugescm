// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
)

type PatchOptions struct {
	Algorithm diferenco.Algorithm
	Textconv  bool
	Match     func(string) bool
}

func sizeOverflow(f *File) bool {
	return f != nil && f.Size > diferenco.MAX_DIFF_SIZE
}

func PathRenameCombine(from, to string) string {
	fromPaths := strings.Split(from, "/")
	toPaths := strings.Split(to, "/")
	n := min(len(fromPaths), len(toPaths))
	i := 0
	for i < n && fromPaths[i] == toPaths[i] {
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%s => %s", from, to)
	}
	return fmt.Sprintf("%s/{%s => %s}", path.Join(fromPaths[0:i]...), path.Join(fromPaths[i:]...), path.Join(toPaths[i:]...))
}

func fileStatName(from, to *File) string {
	if from == nil {
		// New File is created.
		return to.Path
	}
	if to == nil {
		// File is deleted.
		return from.Path
	}
	if from.Path != to.Path {
		// File is renamed.
		return PathRenameCombine(from.Path, to.Path)
	}
	return from.Path
}

func fileStatWithContext(ctx context.Context, opts *PatchOptions, c *Change) (*FileStat, error) {
	from, to, err := c.Files()
	if err != nil {
		return nil, err
	}
	if from == nil && to == nil {
		return nil, ErrMalformedChange
	}
	s := &FileStat{
		Name: fileStatName(from, to),
	}
	if from.IsFragments() || to.IsFragments() {
		return s, nil
	}
	// --- check size limit
	if sizeOverflow(from) || sizeOverflow(to) {
		return s, nil
	}
	fromContent, err := from.UnifiedText(ctx, opts.Textconv)
	if plumbing.IsNoSuchObject(err) || errors.Is(err, diferenco.ErrBinaryData) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	toContent, err := to.UnifiedText(ctx, opts.Textconv)
	if plumbing.IsNoSuchObject(err) || errors.Is(err, diferenco.ErrBinaryData) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	stat, err := diferenco.Stat(ctx, &diferenco.Options{S1: fromContent, S2: toContent, A: opts.Algorithm})
	if err != nil {
		return nil, err
	}
	s.Addition = stat.Addition
	s.Deletion = stat.Deletion
	return s, nil
}

func getStatsContext(ctx context.Context, opts *PatchOptions, changes ...*Change) ([]FileStat, error) {
	if opts.Match == nil {
		opts.Match = func(s string) bool {
			return true
		}
	}
	stats := make([]FileStat, 0, 100)
	for _, c := range changes {
		if !opts.Match(c.name()) {
			continue
		}
		s, err := fileStatWithContext(ctx, opts, c)
		if err != nil {
			return nil, err
		}
		stats = append(stats, *s)
	}
	return stats, nil
}

func filePatchWithContext(ctx context.Context, opts *PatchOptions, c *Change) (*diferenco.Unified, error) {
	from, to, err := c.Files()
	if err != nil {
		return nil, err
	}
	if from == nil && to == nil {
		return nil, ErrMalformedChange
	}
	if from.IsFragments() || to.IsFragments() {
		return &diferenco.Unified{From: from.asFile(), To: to.asFile(), IsFragments: true}, nil
	}
	// --- check size limit
	if sizeOverflow(from) || sizeOverflow(to) {
		return &diferenco.Unified{From: from.asFile(), To: to.asFile(), IsBinary: true}, nil
	}
	fromContent, err := from.UnifiedText(ctx, opts.Textconv)
	if plumbing.IsNoSuchObject(err) || errors.Is(err, diferenco.ErrBinaryData) {
		return &diferenco.Unified{From: from.asFile(), To: to.asFile(), IsBinary: true}, nil
	}
	if err != nil {
		return nil, err
	}
	toContent, err := to.UnifiedText(ctx, opts.Textconv)
	if plumbing.IsNoSuchObject(err) || errors.Is(err, diferenco.ErrBinaryData) {
		return &diferenco.Unified{From: from.asFile(), To: to.asFile(), IsBinary: true}, nil
	}
	if err != nil {
		return nil, err
	}
	return diferenco.DoUnified(ctx, &diferenco.Options{From: from.asFile(), To: to.asFile(), S1: fromContent, S2: toContent, A: opts.Algorithm})
}

func getPatchContext(ctx context.Context, opts *PatchOptions, changes ...*Change) ([]*diferenco.Unified, error) {
	if opts.Match == nil {
		opts.Match = func(s string) bool {
			return true
		}
	}
	patch := make([]*diferenco.Unified, 0, len(changes))
	for _, c := range changes {
		if !opts.Match(c.name()) {
			continue
		}
		p, err := filePatchWithContext(ctx, opts, c)
		if err != nil {
			return nil, err
		}
		patch = append(patch, p)
	}
	return patch, nil
}

// FileStat stores the status of changes in content of a file.
type FileStat struct {
	Name     string `json:"name"`
	Addition int    `json:"addition"`
	Deletion int    `json:"deletion"`
}

func (fs FileStat) String() string {
	var b strings.Builder
	StatsWriteTo(&b, []FileStat{fs}, false)
	return b.String()
}

// FileStats is a collection of FileStat.
type FileStats []FileStat

func (fileStats FileStats) String() string {
	var b strings.Builder
	StatsWriteTo(&b, fileStats, false)
	return b.String()
}

// StatsWriteTo prints the stats of changes in content of files.
// Original implementation: https://github.com/git/git/blob/1a87c842ece327d03d08096395969aca5e0a6996/diff.c#L2615
// Parts of the output:
// <pad><filename><pad>|<pad><changeNumber><pad><+++/---><newline>
// example: " main.go | 10 +++++++--- "
func StatsWriteTo(w io.Writer, fileStats []FileStat, isColorSupported bool) {
	maxGraphWidth := uint(53)
	maxNameLen := 0
	maxChangeLen := 0

	scaleLinear := func(it, width, max uint) uint {
		if it == 0 || max == 0 {
			return 0
		}

		return 1 + (it * (width - 1) / max)
	}

	for _, fs := range fileStats {
		if len(fs.Name) > maxNameLen {
			maxNameLen = len(fs.Name)
		}

		changes := strconv.Itoa(fs.Addition + fs.Deletion)
		if len(changes) > maxChangeLen {
			maxChangeLen = len(changes)
		}
	}
	for _, fs := range fileStats {
		add := uint(fs.Addition)
		del := uint(fs.Deletion)
		np := maxNameLen - len(fs.Name)
		cp := maxChangeLen - len(strconv.Itoa(fs.Addition+fs.Deletion))

		total := add + del
		if total > maxGraphWidth {
			add = scaleLinear(add, maxGraphWidth, total)
			del = scaleLinear(del, maxGraphWidth, total)
		}

		adds := strings.Repeat("+", int(add))
		dels := strings.Repeat("-", int(del))
		namePad := strings.Repeat(" ", np)
		changePad := strings.Repeat(" ", cp)
		if isColorSupported {
			_, _ = fmt.Fprintf(w, " %s%s | %s%d \x1b[32m%s\x1b[31m%s\x1b[0m\n", fs.Name, namePad, changePad, total, adds, dels)
			continue
		}
		_, _ = fmt.Fprintf(w, " %s%s | %s%d %s%s\n", fs.Name, namePad, changePad, total, adds, dels)
	}
}
