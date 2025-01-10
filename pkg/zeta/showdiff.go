package zeta

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/diferenco/color"
	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type DiffOptions struct {
	NameOnly   bool
	NameStatus bool // name status
	Numstat    bool
	Stat       bool
	Shortstat  bool
	Staged     bool
	NewLine    byte
	NewOutput  func(context.Context) (Printer, error) // new writer func
	NoRename   bool
	// index value
	MergeBase string
	From      string
	To        string
	PathSpec  []string
	Textconv  bool
	UseColor  bool
	Way3      bool
	Algorithm diferenco.Algorithm
}

func (opts *DiffOptions) po() *object.PatchOptions {
	m := NewMatcher(opts.PathSpec)
	return &object.PatchOptions{Textconv: opts.Textconv, Algorithm: opts.Algorithm, Match: m.Match}
}

func (opts *DiffOptions) ShowChanges(ctx context.Context, changes object.Changes) error {
	if opts.NameOnly {
		return opts.showNameOnly(ctx, changes)
	}
	if opts.NameStatus {
		return opts.showNameStatus(ctx, changes)
	}
	if opts.showStatOnly() {
		fileStats, err := changes.Stats(ctx, opts.po())
		if err != nil {
			return err
		}
		return opts.ShowStats(ctx, fileStats)
	}
	patch, err := changes.Patch(ctx, opts.po())
	if err != nil {
		return err
	}
	return opts.ShowPatch(ctx, patch)
}

func (opts *DiffOptions) showNameOnly(ctx context.Context, changes object.Changes) error {
	w, err := opts.NewOutput(ctx)
	if err != nil {
		return err
	}
	defer w.Close()
	m := NewMatcher(opts.PathSpec)
	for _, c := range changes {
		name := c.Name()
		if !m.Match(name) {
			continue
		}
		fmt.Fprintf(w, "%s%c", name, opts.NewLine)
	}
	return nil
}

func changeStat(c *object.Change) (string, byte) {
	action, err := c.Action()
	if err != nil {
		return "", ' '
	}
	switch action {
	case merkletrie.Insert:
		return c.To.Name, 'A'
	case merkletrie.Delete:
		return c.From.Name, 'D'
	case merkletrie.Modify:
		if c.From.Name != c.To.Name {
			return c.From.Name, 'R'
		}
		return c.From.Name, 'M'
	}
	return "", ' '
}

func (opts *DiffOptions) showNameStatus(ctx context.Context, changes object.Changes) error {
	w, err := opts.NewOutput(ctx)
	if err != nil {
		return err
	}
	defer w.Close()
	m := NewMatcher(opts.PathSpec)
	for _, c := range changes {
		name, stat := changeStat(c)
		if !m.Match(name) {
			continue
		}
		fmt.Fprintf(w, "%c    %s%c", stat, name, opts.NewLine)
	}
	return nil
}

func (opts *DiffOptions) showStatOnly() bool {
	return opts.Numstat || opts.Stat || opts.Shortstat
}

func numPadding(i int, padding int) string {
	s := strconv.Itoa(i)
	if len(s) >= padding {
		return s
	}
	return s + strings.Repeat(" ", padding-len(s))
}

func numPaddingLeft(i int, padding int) string {
	s := strconv.Itoa(i)
	if len(s) >= padding {
		return s
	}
	return strings.Repeat(" ", padding-len(s)) + s
}

// ShowStats: show stats
//
// Original implementation: https://github.com/git/git/blob/1a87c842ece327d03d08096395969aca5e0a6996/diff.c#L2615
// Parts of the output:
// <pad><filename><pad>|<pad><changeNumber><pad><+++/---><newline>
// example: " main.go | 10 +++++++--- "
func (opts *DiffOptions) ShowStats(ctx context.Context, fileStats object.FileStats) error {
	w, err := opts.NewOutput(ctx)
	if err != nil {
		return err
	}
	defer w.Close()
	if opts.Shortstat {
		var added, deleted int
		for _, s := range fileStats {
			added += s.Addition
			deleted += s.Deletion
		}
		fmt.Fprintf(w, " %d files changed, %d insertions(+), %d deletions(-)%c", len(fileStats), added, deleted, opts.NewLine)
		return nil
	}
	if opts.Numstat {
		var ma, md int
		for _, s := range fileStats {
			ma = max(ma, s.Addition)
			md = max(md, s.Deletion)
		}
		addPadding := len(strconv.Itoa(ma)) + 4
		deletePadding := len(strconv.Itoa(md)) + 4
		for _, s := range fileStats {
			fmt.Fprintf(w, "%s %s %s%c", numPadding(s.Addition, addPadding), numPadding(s.Deletion, deletePadding), s.Name, opts.NewLine)
		}
		return nil
	}
	var added, deleted int
	var nameLen, modified int
	for _, s := range fileStats {
		added += s.Addition
		deleted += s.Deletion
		nameLen = max(nameLen, len(s.Name))
		modified = max(modified, s.Addition+s.Deletion)
	}
	scaleFactor := 1.0
	sizePadding := len(strconv.Itoa(modified))
	for _, fs := range fileStats {
		addn := float64(fs.Addition)
		deln := float64(fs.Deletion)
		addc := int(math.Floor(addn / scaleFactor))
		delc := int(math.Floor(deln / scaleFactor))
		if addc < 0 {
			addc = 0
		}
		if delc < 0 {
			delc = 0
		}
		adds := strings.Repeat("+", addc)
		dels := strings.Repeat("-", delc)
		if w.ColorMode() != term.LevelNone {
			_, _ = fmt.Fprintf(w, " %s%s | %s \x1b[32m%s\x1b[31m%s\x1b[0m\n", fs.Name, strings.Repeat(" ", nameLen-len(fs.Name)), numPaddingLeft(fs.Addition+fs.Deletion, sizePadding), adds, dels)
			continue
		}
		fmt.Fprintf(w, "%s%s | %s %s%s%c", fs.Name, strings.Repeat(" ", nameLen-len(fs.Name)), numPaddingLeft(fs.Addition+fs.Deletion, sizePadding), adds, dels, opts.NewLine)
	}
	fmt.Fprintf(w, " %d files changed, %d insertions(+), %d deletions(-)%c", len(fileStats), added, deleted, opts.NewLine)
	return nil
}

func (opts *DiffOptions) ShowPatch(ctx context.Context, patch []*diferenco.Unified) error {
	w, err := opts.NewOutput(ctx)
	if err != nil {
		return err
	}
	defer w.Close()
	e := diferenco.NewUnifiedEncoder(w)
	if w.ColorMode() != term.LevelNone {
		e.SetColor(color.NewColorConfig())
	}
	if opts.NoRename {
		e.SetNoRename()
	}
	_ = e.Encode(patch)
	return nil
}

func (opts *DiffOptions) showChangesStatus(ctx context.Context, changes merkletrie.Changes) error {
	w, err := opts.NewOutput(ctx)
	if err != nil {
		return err
	}
	defer w.Close()
	m := NewMatcher(opts.PathSpec)
	if opts.NameOnly {
		for _, c := range changes {
			name := nameFromAction(&c)
			if !m.Match(name) {
				continue
			}
			fmt.Fprintf(w, "%s%c", name, opts.NewLine)
		}
		return nil
	}
	// name-status
	for _, c := range changes {
		name := nameFromAction(&c)
		if !m.Match(name) {
			continue
		}
		a, err := c.Action()
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%c    %s%c", a.Byte(), name, opts.NewLine)
	}
	return nil
}
