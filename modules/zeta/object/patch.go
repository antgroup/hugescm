// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/diff"
	dmp "github.com/antgroup/hugescm/modules/diffmatchpatch"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	fdiff "github.com/antgroup/hugescm/modules/plumbing/format/diff"
)

var (
	ErrCanceled = errors.New("operation canceled")
)

func getPatchContext(ctx context.Context, message string, codecvt bool, changes ...*Change) (*Patch, error) {
	var filePatches []fdiff.FilePatch
	for _, c := range changes {
		select {
		case <-ctx.Done():
			return nil, ErrCanceled
		default:
		}

		fp, err := filePatchWithContext(ctx, codecvt, c)
		if err != nil {
			return nil, err
		}

		filePatches = append(filePatches, fp)
	}
	return &Patch{message, filePatches}, nil
}

func sizeOverflow(f *File) bool {
	return f != nil && f.Size > MAX_DIFF_SIZE
}

func filePatchWithContext(ctx context.Context, codecvt bool, c *Change) (fdiff.FilePatch, error) {
	if c.From.IsFragments() || c.To.IsFragments() {
		return &textFilePatch{from: c.From, to: c.To, fragments: true}, nil
	}
	from, to, err := c.Files()
	if err != nil {
		return nil, err
	}
	// --- check size limit
	if sizeOverflow(from) || sizeOverflow(to) {
		return &textFilePatch{from: c.From, to: c.To}, nil
	}
	fromContent, err := from.UnifiedText(ctx, codecvt)
	if plumbing.IsNoSuchObject(err) {
		return &textFilePatch{from: c.From, to: c.To}, nil
	}
	if err == ErrNotTextContent {
		return &textFilePatch{from: c.From, to: c.To}, nil
	}
	if err != nil {
		return nil, err
	}
	toContent, err := to.UnifiedText(ctx, codecvt)
	if plumbing.IsNoSuchObject(err) {
		return &textFilePatch{from: c.From, to: c.To}, nil
	}
	if err == ErrNotTextContent {
		return &textFilePatch{from: c.From, to: c.To}, nil
	}
	if err != nil {
		return nil, err
	}

	diffs, err := diff.Do(fromContent, toContent)
	if err != nil {
		return &textFilePatch{from: c.From, to: c.To}, nil
	}
	var chunks []fdiff.Chunk
	for _, d := range diffs {
		select {
		case <-ctx.Done():
			return nil, ErrCanceled
		default:
		}

		var op fdiff.Operation
		switch d.Type {
		case dmp.DiffEqual:
			op = fdiff.Equal
		case dmp.DiffDelete:
			op = fdiff.Delete
		case dmp.DiffInsert:
			op = fdiff.Add
		}

		chunks = append(chunks, &textChunk{d.Text, op})
	}

	return &textFilePatch{
		chunks: chunks,
		from:   c.From,
		to:     c.To,
	}, nil

}

// Patch is an implementation of fdiff.Patch interface
type Patch struct {
	message     string
	filePatches []fdiff.FilePatch
}

func NewPatch(message string, filePatches []fdiff.FilePatch) *Patch {
	return &Patch{message: message, filePatches: filePatches}
}

func (p *Patch) FilePatches() []fdiff.FilePatch {
	return p.filePatches
}

func (p *Patch) Message() string {
	return p.message
}

func (p *Patch) Encode(w io.Writer) error {
	e := fdiff.NewUnifiedEncoder(w, fdiff.DefaultContextLines)

	return e.Encode(p)
}

func (p *Patch) EncodeEx(w io.Writer, useColor bool) error {
	e := fdiff.NewUnifiedEncoder(w, fdiff.DefaultContextLines)
	if useColor {
		e.SetColor(fdiff.NewColorConfig())
	}
	return e.Encode(p)
}

func (p *Patch) Stats() FileStats {
	return getFileStatsFromFilePatches(p.FilePatches())
}

func (p *Patch) String() string {
	buf := bytes.NewBuffer(nil)
	err := p.Encode(buf)
	if err != nil {
		return fmt.Sprintf("malformed patch: %s", err.Error())
	}

	return buf.String()
}

// changeEntryWrapper is an implementation of fdiff.File interface
type changeEntryWrapper struct {
	ce ChangeEntry
}

func (f *changeEntryWrapper) Hash() plumbing.Hash {
	if !f.ce.TreeEntry.Mode.IsFile() {
		return plumbing.ZeroHash
	}

	return f.ce.TreeEntry.Hash
}

func (f *changeEntryWrapper) Mode() filemode.FileMode {
	return f.ce.TreeEntry.Mode.Origin()
}
func (f *changeEntryWrapper) Path() string {
	if !f.ce.TreeEntry.Mode.IsFile() {
		return ""
	}

	return f.ce.Name
}

func (f *changeEntryWrapper) Empty() bool {
	return !f.ce.TreeEntry.Mode.IsFile()
}

// textFilePatch is an implementation of fdiff.FilePatch interface
type textFilePatch struct {
	chunks    []fdiff.Chunk
	from, to  ChangeEntry
	fragments bool
}

func NewTextFilePatch(chunks []fdiff.Chunk, from, to ChangeEntry, fragments bool) fdiff.FilePatch {
	return &textFilePatch{chunks: chunks, from: from, to: to, fragments: fragments}
}

func (tf *textFilePatch) Files() (from fdiff.File, to fdiff.File) {
	f := &changeEntryWrapper{tf.from}
	t := &changeEntryWrapper{tf.to}

	if !f.Empty() {
		from = f
	}

	if !t.Empty() {
		to = t
	}

	return
}

func (tf *textFilePatch) IsFragments() bool {
	return tf.fragments
}

func (tf *textFilePatch) IsBinary() bool {
	return len(tf.chunks) == 0
}

func (tf *textFilePatch) Chunks() []fdiff.Chunk {
	return tf.chunks
}

type filePatchWrapper struct {
	chunks    []fdiff.Chunk
	from, to  fdiff.File
	fragments bool
}

func (f *filePatchWrapper) Files() (from fdiff.File, to fdiff.File) {
	from = f.from
	to = f.to

	return
}

func (f *filePatchWrapper) IsFragments() bool {
	return f.fragments
}

func (f *filePatchWrapper) IsBinary() bool {
	return len(f.chunks) == 0
}

func (f *filePatchWrapper) Chunks() []fdiff.Chunk {
	return f.chunks
}

func NewFilePatchWrapper(chunks []fdiff.Chunk, from, to fdiff.File, fragments bool) fdiff.FilePatch {
	return &filePatchWrapper{chunks: chunks, from: from, to: to, fragments: fragments}
}

// textChunk is an implementation of fdiff.Chunk interface
type textChunk struct {
	content string
	op      fdiff.Operation
}

func (t *textChunk) Content() string {
	return t.content
}

func (t *textChunk) Type() fdiff.Operation {
	return t.op
}

func NewTextChunk(content string, op fdiff.Operation) fdiff.Chunk {
	return &textChunk{content: content, op: op}
}

// FileStat stores the status of changes in content of a file.
type FileStat struct {
	Name     string
	Addition int
	Deletion int
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
func StatsWriteTo(w io.Writer, fileStats []FileStat, color bool) {
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
		if color {
			_, _ = fmt.Fprintf(w, " %s%s | %s%d \x1b[32m%s\x1b[31m%s\x1b[0m\n", fs.Name, namePad, changePad, total, adds, dels)
			continue
		}
		_, _ = fmt.Fprintf(w, " %s%s | %s%d %s%s\n", fs.Name, namePad, changePad, total, adds, dels)
	}
}

func getFileStatsFromFilePatches(filePatches []fdiff.FilePatch) FileStats {
	var fileStats FileStats

	for _, fp := range filePatches {
		// ignore empty patches (binary files, submodule refs updates)
		if len(fp.Chunks()) == 0 {
			continue
		}

		cs := FileStat{}
		from, to := fp.Files()
		if from == nil {
			// New File is created.
			cs.Name = to.Path()
		} else if to == nil {
			// File is deleted.
			cs.Name = from.Path()
		} else if from.Path() != to.Path() {
			// File is renamed.
			cs.Name = fmt.Sprintf("%s => %s", from.Path(), to.Path())
		} else {
			cs.Name = from.Path()
		}

		for _, chunk := range fp.Chunks() {
			s := chunk.Content()
			if len(s) == 0 {
				continue
			}

			switch chunk.Type() {
			case fdiff.Add:
				cs.Addition += strings.Count(s, "\n")
				if s[len(s)-1] != '\n' {
					cs.Addition++
				}
			case fdiff.Delete:
				cs.Deletion += strings.Count(s, "\n")
				if s[len(s)-1] != '\n' {
					cs.Deletion++
				}
			}
		}

		fileStats = append(fileStats, cs)
	}

	return fileStats
}
