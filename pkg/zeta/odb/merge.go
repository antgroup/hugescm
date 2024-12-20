// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"path"
	"runtime"
	"sort"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/tr"
)

const (
	mergeLimit = 50 * 1024 * 1024 // 50M
)

// ConflictEntry represents a conflict entry which is one of the sides of a conflict.
type ConflictEntry struct {
	// Path is the path of the conflicting file.
	Path string `json:"path"`
	// Mode is the mode of the conflicting file.
	Mode filemode.FileMode `json:"mode"`
	Hash plumbing.Hash     `json:"oid"`
}

const (
	INFO_AUTO_MERGING = iota
	CONFLICT_CONTENTS
	CONFLICT_BINARY
	CONFLICT_FILE_DIRECTORY
	CONFLICT_DISTINCT_MODES
	CONFLICT_MODIFY_DELETE
	// Regular rename
	CONFLICT_RENAME_RENAME
	CONFLICT_RENAME_COLLIDES
	CONFLICT_RENAME_DELETE
	CONFLICT_DIR_RENAME_SUGGESTED
	INFO_DIR_RENAME_APPLIED
	// Special directory rename cases
	INFO_DIR_RENAME_SKIPPED_DUE_TO_RERENAME
	CONFLICT_DIR_RENAME_FILE_IN_WAY
	CONFLICT_DIR_RENAME_COLLISION
	CONFLICT_DIR_RENAME_SPLIT
)

// var (
// 	mergeDescription = map[int]string{
// 		/*** "Simple" conflicts and informational messages ***/
// 		INFO_AUTO_MERGING:       "Auto-merging",
// 		CONFLICT_CONTENTS:       "CONFLICT (contents)",
// 		CONFLICT_BINARY:         "CONFLICT (binary)",
// 		CONFLICT_FILE_DIRECTORY: "CONFLICT (file/directory)",
// 		CONFLICT_DISTINCT_MODES: "CONFLICT (distinct modes)",
// 		CONFLICT_MODIFY_DELETE:  "CONFLICT (modify/delete)",
// 		/*** Regular rename ***/
// 		CONFLICT_RENAME_RENAME:   "CONFLICT (rename/rename)",
// 		CONFLICT_RENAME_COLLIDES: "CONFLICT (rename involved in collision)",
// 		CONFLICT_RENAME_DELETE:   "CONFLICT (rename/delete)",

// 		/*** Basic directory rename ***/
// 		CONFLICT_DIR_RENAME_SUGGESTED: "CONFLICT (directory rename suggested)",
// 		INFO_DIR_RENAME_APPLIED:       "Path updated due to directory rename",

// 		/*** Special directory rename cases ***/
// 		INFO_DIR_RENAME_SKIPPED_DUE_TO_RERENAME: "Directory rename skipped since directory was renamed on both sides",
// 		CONFLICT_DIR_RENAME_FILE_IN_WAY:         "CONFLICT (file in way of directory rename)",
// 		CONFLICT_DIR_RENAME_COLLISION:           "CONFLICT(directory rename collision)",
// 		CONFLICT_DIR_RENAME_SPLIT:               "CONFLICT(directory rename unclear split)",
// 	}
// )

// Conflict represents a merge conflict for a single file.
type Conflict struct {
	// Ancestor is the conflict entry of the merge-base.
	Ancestor ConflictEntry `json:"ancestor"`
	// Our is the conflict entry of ours.
	Our ConflictEntry `json:"our"`
	// Their is the conflict entry of theirs.
	Their ConflictEntry `json:"their"`
	// Types: conflict types
	Types int `json:"types"`
}

type ChangeEntry struct {
	Path     string
	Ancestor *object.TreeEntry
	Our      *object.TreeEntry
	Their    *object.TreeEntry
}

func (e *ChangeEntry) replace(newName string) *ChangeEntry {
	newEntry := &ChangeEntry{Path: newName, Ancestor: e.Ancestor, Our: e.Our, Their: e.Their}
	baseName := path.Base(newName)
	if newEntry.Our != nil {
		newEntry.Our.Name = baseName
	}
	if newEntry.Their != nil {
		newEntry.Their.Name = baseName
	}
	return newEntry
}

func (e *ChangeEntry) modifiedEntry() *TreeEntry {
	if e.Our != nil {
		return &TreeEntry{Path: e.Path, TreeEntry: e.Our}
	}
	return &TreeEntry{Path: e.Path, TreeEntry: e.Their}
}

func (e *ChangeEntry) conflictMode() (filemode.FileMode, bool) {
	if e.Ancestor.Mode == e.Our.Mode {
		return e.Their.Mode, false
	}
	if e.Ancestor.Mode == e.Their.Mode {
		return e.Our.Mode, false
	}
	return e.Our.Mode, e.Our.Mode == e.Their.Mode
}

func (e *ChangeEntry) hasConflict() bool {
	// !(their modified|our modified|our equal their: delete both or insert both)
	return !(e.Ancestor.Equal(e.Our) || e.Ancestor.Equal(e.Their) || e.Our.Equal(e.Their))
}

func (e *ChangeEntry) makeConflict(side int) *Conflict {
	c := &Conflict{Types: side}
	if e.Ancestor != nil {
		c.Ancestor.Hash = e.Ancestor.Hash
		c.Ancestor.Mode = e.Ancestor.Mode
		c.Ancestor.Path = e.Path
	}
	if e.Our != nil {
		c.Our.Hash = e.Our.Hash
		c.Our.Mode = e.Our.Mode
		c.Our.Path = e.Path
	}
	if e.Their != nil {
		c.Their.Hash = e.Their.Hash
		c.Their.Mode = e.Their.Mode
		c.Their.Path = e.Path
	}
	return c
}

type RenameEntry struct {
	Ancestor *TreeEntry
	Our      *TreeEntry
	Their    *TreeEntry
}

func (e *RenameEntry) conflict() bool {
	// !(their rename|our rename|both rename equal)
	return !(e.Our == nil || e.Their == nil || e.Our.Equal(e.Their))
}

func (e *RenameEntry) makeConflict() *Conflict {
	c := &Conflict{
		Ancestor: ConflictEntry{
			Path: e.Ancestor.Path,
			Mode: e.Ancestor.Mode,
			Hash: e.Ancestor.Hash,
		},
		Types: CONFLICT_RENAME_RENAME,
	}
	if e.Our != nil {
		c.Our = ConflictEntry{
			Path: e.Our.Path,
			Mode: e.Our.Mode,
			Hash: e.Our.Hash,
		}
	}
	if e.Their != nil {
		c.Their = ConflictEntry{
			Path: e.Their.Path,
			Mode: e.Their.Mode,
			Hash: e.Their.Hash,
		}
	}
	return c
}

type differences struct {
	entries map[string]*ChangeEntry
	// rename
	renames map[string]*RenameEntry
	ours    map[string]bool
	theirs  map[string]bool
}

func (d *differences) overrideOur(ch *object.Change, action merkletrie.Action) {
	if action == merkletrie.Insert {
		d.ours[ch.To.Name] = true
		d.entries[ch.To.Name] = &ChangeEntry{Path: ch.To.Name, Our: &ch.To.TreeEntry}
		return
	}
	if action == merkletrie.Delete {
		d.entries[ch.From.Name] = &ChangeEntry{Path: ch.From.Name, Ancestor: &ch.From.TreeEntry, Their: &ch.From.TreeEntry}
		return
	}
	d.ours[ch.To.Name] = true
	if ch.From.Name == ch.To.Name {
		d.entries[ch.From.Name] = &ChangeEntry{Path: ch.From.Name, Ancestor: &ch.From.TreeEntry, Our: &ch.To.TreeEntry, Their: &ch.From.TreeEntry}
		return
	}
	// rename style
	d.renames[ch.From.Name] = &RenameEntry{
		Ancestor: &TreeEntry{Path: ch.From.Name, TreeEntry: &ch.From.TreeEntry},
		Our:      &TreeEntry{Path: ch.To.Name, TreeEntry: &ch.To.TreeEntry},
	}
	d.entries[ch.From.Name] = &ChangeEntry{Path: ch.From.Name, Ancestor: &ch.From.TreeEntry, Their: &ch.From.TreeEntry}
	d.entries[ch.To.Name] = &ChangeEntry{Path: ch.To.Name, Our: &ch.To.TreeEntry}
}

func (d *differences) overrideTheir(ch *object.Change, action merkletrie.Action) {
	if action == merkletrie.Insert {
		d.theirs[ch.To.Name] = true
		if e, ok := d.entries[ch.To.Name]; ok {
			e.Their = &ch.To.TreeEntry
			return
		}
		d.entries[ch.To.Name] = &ChangeEntry{Path: ch.To.Name, Their: &ch.To.TreeEntry}
		return
	}
	if action == merkletrie.Delete {
		if e, ok := d.entries[ch.From.Name]; ok {
			e.Their = nil
			return
		}
		d.entries[ch.From.Name] = &ChangeEntry{Path: ch.From.Name, Ancestor: &ch.From.TreeEntry, Our: &ch.From.TreeEntry}
		return
	}
	d.theirs[ch.To.Name] = true
	if ch.From.Name == ch.To.Name {
		if e, ok := d.entries[ch.From.Name]; ok {
			e.Their = &ch.To.TreeEntry
			return
		}
		d.entries[ch.From.Name] = &ChangeEntry{Path: ch.From.Name, Ancestor: &ch.From.TreeEntry, Our: &ch.From.TreeEntry, Their: &ch.To.TreeEntry}
		return
	}
	if e, ok := d.renames[ch.From.Name]; ok {
		e.Their = &TreeEntry{Path: ch.To.Name, TreeEntry: &ch.To.TreeEntry}
	} else {
		d.renames[ch.From.Name] = &RenameEntry{
			Ancestor: &TreeEntry{Path: ch.From.Name, TreeEntry: &ch.From.TreeEntry},
			Their:    &TreeEntry{Path: ch.To.Name, TreeEntry: &ch.To.TreeEntry},
		}
	}
	// rename style: delete old
	if e, ok := d.entries[ch.From.Name]; ok {
		e.Their = nil
	} else {
		d.entries[ch.From.Name] = &ChangeEntry{Path: ch.From.Name, Ancestor: &ch.From.TreeEntry, Our: &ch.From.TreeEntry}
	}
	// insert new
	if e, ok := d.entries[ch.To.Name]; ok {
		e.Their = &ch.To.TreeEntry
	} else {
		d.entries[ch.To.Name] = &ChangeEntry{Path: ch.To.Name, Their: &ch.To.TreeEntry}
	}
}

func (d *differences) nameConflicts() map[string]string {
	names := make([]string, 0, len(d.entries))
	for p := range d.entries {
		names = append(names, p)
	}
	conflicts := make(map[string]string)
	sort.Strings(names)
	for i := 0; i < len(names); i++ {
		prefix := names[i] + "/"
		for j := i + 1; j < len(names); j++ {
			if strings.HasPrefix(names[j], prefix) {
				conflicts[names[i]] = names[j]
			}
		}
	}
	return conflicts
}

func (d *ODB) mergeDifferences(ctx context.Context, o, a, b *object.Tree) (*differences, error) {
	m := noder.NewSparseTreeMatcher(nil)
	opts := &object.DiffTreeOptions{
		DetectRenames:    true,
		OnlyExactRenames: true,
	}
	ours, err := object.DiffTreeWithOptions(ctx, o, a, opts, m)
	if err != nil {
		return nil, err
	}
	theirs, err := object.DiffTreeWithOptions(ctx, o, b, opts, m)
	if err != nil {
		return nil, err
	}
	ds := &differences{
		entries: make(map[string]*ChangeEntry),
		renames: make(map[string]*RenameEntry),
		ours:    make(map[string]bool),
		theirs:  make(map[string]bool),
	}
	for _, c := range ours {
		action, err := c.Action()
		if err != nil {
			return nil, err
		}
		ds.overrideOur(c, action)
	}
	for _, c := range theirs {
		action, err := c.Action()
		if err != nil {
			return nil, err
		}
		ds.overrideTheir(c, action)
	}
	return ds, nil
}

const (
	MERGE_VARIANT_NORMAL = 0
	MERGE_VARIANT_OURS   = 1
	MERGE_VARIANT_THEIRS = 2
)

type MergeOptions struct {
	Branch1       string
	Branch2       string
	DetectRenames bool
	RenameLimit   int
	RenameScore   int
	Variant       int
	Textconv      bool
	MergeDriver   MergeDriver
	TextGetter    TextGetter
}

type MergeResult struct {
	NewTree   plumbing.Hash `json:"new-tree"`
	Conflicts []*Conflict   `json:"conflicts,omitempty"`
	Messages  []string      `json:"messages,omitempty"`
}

func (mr *MergeResult) Error() string {
	return "conflicts"
}

func (d *ODB) mergeEntry(ctx context.Context, ch *ChangeEntry, opts *MergeOptions, result *MergeResult) (*TreeEntry, error) {
	// Both sides add
	if ch.Ancestor == nil {
		switch {
		case ch.Our.Hash == ch.Their.Hash:
			// Only filemode changes
			result.Messages = append(result.Messages, tr.Sprintf("CONFLICT (distinct types): %s had different types on each side; renamed both of them so each can be recorded somewhere.", ch.Path))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_DISTINCT_MODES))
			return &TreeEntry{Path: ch.Path, TreeEntry: ch.Our}, nil
		case ch.Our.IsFragments() || ch.Their.IsFragments() || ch.Our.Size > mergeLimit || ch.Their.Size > mergeLimit:
			result.Messages = append(result.Messages, tr.Sprintf("warning: Cannot merge binary files: %s (%s vs. %s)", ch.Path, opts.Branch1, opts.Branch2))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_BINARY))
			return &TreeEntry{Path: ch.Path, TreeEntry: ch.Our}, nil
		default:
		}
		mr, err := d.mergeText(ctx, &mergeOptions{
			O:        backend.BLANK_BLOB_HASH, // empty blob
			A:        ch.Our.Hash,
			B:        ch.Their.Hash,
			LabelO:   "",
			LableA:   ch.Path,
			LabelB:   ch.Path,
			Textconv: opts.Textconv,
			M:        opts.MergeDriver,
			G:        opts.TextGetter,
		})
		if err == diferenco.ErrNonTextContent {
			result.Messages = append(result.Messages, tr.Sprintf("warning: Cannot merge binary files: %s (%s vs. %s)", ch.Path, opts.Branch1, opts.Branch2))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_BINARY))
			return &TreeEntry{Path: ch.Path, TreeEntry: ch.Our}, nil
		}
		if err != nil {
			return nil, err
		}
		if mr.conflict {
			// Note: If there is no automatic encoding conversion, conflicts will definitely occur when merging here.
			result.Messages = append(result.Messages, tr.Sprintf("CONFLICT (%s): Merge conflict in %s", tr.W("add/add"), ch.Path))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_CONTENTS))
		}
		return &TreeEntry{
			Path: ch.Path,
			TreeEntry: &object.TreeEntry{
				Name: ch.Our.Name,
				Size: mr.size,
				Mode: ch.Our.Mode,
				Hash: mr.oid,
			}}, nil
	}
	// Modifications by both parties:
	if ch.Our != nil && ch.Their != nil {
		switch {
		case ch.Our.Hash == ch.Their.Hash:
			// Only filemode changes
			result.Messages = append(result.Messages, tr.Sprintf("CONFLICT (distinct types): %s had different types on each side; renamed both of them so each can be recorded somewhere.", ch.Path))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_DISTINCT_MODES))
			return &TreeEntry{Path: ch.Path, TreeEntry: ch.Our}, nil
		case ch.Our.IsFragments() || ch.Their.IsFragments() || ch.Our.Size > mergeLimit || ch.Their.Size > mergeLimit:
			result.Messages = append(result.Messages, tr.Sprintf("warning: Cannot merge binary files: %s (%s vs. %s)", ch.Path, opts.Branch1, opts.Branch2))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_BINARY))
			return &TreeEntry{Path: ch.Path, TreeEntry: ch.Our}, nil
		default:
		}
		mr, err := d.mergeText(ctx,
			&mergeOptions{
				O:        ch.Ancestor.Hash,
				A:        ch.Our.Hash,
				B:        ch.Their.Hash,
				LabelO:   ch.Path,
				LableA:   ch.Path,
				LabelB:   ch.Path,
				Textconv: opts.Textconv,
				M:        opts.MergeDriver,
				G:        opts.TextGetter,
			})
		if err == diferenco.ErrNonTextContent {
			result.Messages = append(result.Messages, tr.Sprintf("warning: Cannot merge binary files: %s (%s vs. %s)", ch.Path, opts.Branch1, opts.Branch2))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_BINARY))
			return &TreeEntry{Path: ch.Path, TreeEntry: ch.Our}, nil
		}
		if err != nil {
			return nil, err
		}
		newMode, modeConflict := ch.conflictMode()
		switch {
		case mr.conflict:
			result.Messages = append(result.Messages, tr.Sprintf("CONFLICT (%s): Merge conflict in %s", tr.W("content"), ch.Path))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_CONTENTS))
		case modeConflict:
			result.Messages = append(result.Messages, tr.Sprintf("CONFLICT (distinct types): %s had different types on each side; renamed both of them so each can be recorded somewhere.", ch.Path))
			result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_DISTINCT_MODES))
		default:
		}
		return &TreeEntry{
			Path: ch.Path,
			TreeEntry: &object.TreeEntry{
				Name: ch.Our.Name,
				Size: mr.size,
				Mode: newMode,
				Hash: mr.oid,
			}}, nil
	}
	// One side deletes, the other side modifies:
	// our modified, theirs delete
	// their modified, our delete
	var message string
	if ch.Our == nil {
		message = tr.Sprintf("CONFLICT (modify/delete): %s deleted in %s and modified in %s.", ch.Path, opts.Branch1, opts.Branch2)
	} else {
		message = tr.Sprintf("CONFLICT (modify/delete): %s deleted in %s and modified in %s.", ch.Path, opts.Branch2, opts.Branch1)
	}
	result.Messages = append(result.Messages, message)
	result.Conflicts = append(result.Conflicts, ch.makeConflict(CONFLICT_MODIFY_DELETE))
	return ch.modifiedEntry(), nil
}

func flatBranchName(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c == '/' || (c == '\\' && runtime.GOOS == "windows") {
			_ = b.WriteByte('_')
			continue
		}
		_, _ = b.WriteRune(c)
	}
	return b.String()
}

func (d *ODB) unifiedText(ctx context.Context, oid plumbing.Hash, textconv bool) (string, string, error) {
	br, err := d.Blob(ctx, oid)
	if err != nil {
		return "", "", err
	}
	defer br.Close()
	return diferenco.ReadUnifiedText(br.Contents, br.Size, textconv)
}

// MergeTree: three way merge tree
func (d *ODB) MergeTree(ctx context.Context, o, a, b *object.Tree, opts *MergeOptions) (*MergeResult, error) {
	if opts.Branch1 == "" {
		opts.Branch1 = "Branch1"
	}
	if opts.Branch2 == "" {
		opts.Branch2 = "Branch2"
	}
	if opts.MergeDriver == nil {
		opts.MergeDriver = diferenco.DefaultMerge // fallback
	}
	if opts.TextGetter == nil {
		opts.TextGetter = d.unifiedText
	}
	diffs, err := d.mergeDifferences(ctx, o, a, b)
	if err != nil {
		return nil, err
	}
	entries, err := d.LsTreeRecurse(ctx, o)
	if err != nil {
		return nil, err
	}
	result := &MergeResult{}
	// check rename conflicts
	for _, e := range diffs.renames {
		if !e.conflict() {
			continue
		}
		result.Messages = append(result.Messages,
			tr.Sprintf("CONFLICT (rename/rename): %s renamed to %s in %s and to %s in %s.", e.Ancestor.Path, e.Our.Path, opts.Branch1, e.Their.Path, opts.Branch2))
		result.Conflicts = append(result.Conflicts, e.makeConflict())
	}
	// check file/directory conflict
	nameConflicts := diffs.nameConflicts()
	for name := range nameConflicts {
		e, ok := diffs.entries[name]
		if !ok {
			continue
		}
		branchName := opts.Branch1
		if diffs.theirs[name] {
			branchName = opts.Branch2
		}
		delete(diffs.entries, name)
		newName := strengthen.StrCat(e.Path, "~", flatBranchName(branchName))
		newEntry := e.replace(newName)
		result.Messages = append(result.Messages,
			tr.Sprintf("CONFLICT (file/directory): directory in the way of %s from %s; moving it to %s instead.", name, branchName, newName))
		result.Conflicts = append(result.Conflicts, newEntry.makeConflict(CONFLICT_FILE_DIRECTORY))
		diffs.entries[newName] = newEntry
	}
	newEntries := make([]*TreeEntry, 0, len(entries))
	for _, e := range entries {
		if _, ok := diffs.entries[e.Path]; !ok {
			newEntries = append(newEntries, e)
			continue
		}
	}

	for _, e := range diffs.entries {
		// ours unmodified
		if e.Ancestor.Equal(e.Our) {
			if e.Their != nil {
				newEntries = append(newEntries, &TreeEntry{Path: e.Path, TreeEntry: e.Their})
			}
			continue
		}
		// theirs unmodified
		if e.Ancestor.Equal(e.Their) {
			if e.Our != nil {
				newEntries = append(newEntries, &TreeEntry{Path: e.Path, TreeEntry: e.Our})
			}
			continue
		}
		// Add same content/delete same files
		if e.Our.Equal(e.Their) {
			if e.Our != nil {
				newEntries = append(newEntries, &TreeEntry{Path: e.Path, TreeEntry: e.Our})
			}
			continue
		}
		result.Messages = append(result.Messages, tr.Sprintf("Auto-merging %s", e.Path))
		mergedEntry, err := d.mergeEntry(ctx, e, opts, result)
		if err != nil {
			return nil, err
		}
		newEntries = append(newEntries, mergedEntry)
	}
	m := &treeMaker{
		ODB: d,
	}

	if result.NewTree, err = m.makeTrees(newEntries); err != nil {
		return nil, err
	}
	return result, nil
}
