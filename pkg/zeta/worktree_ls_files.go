// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

// https://git-scm.com/docs/git-ls-files/zh_HANS-CN
// Show information about files in the index and the working tree

type ListFilesMode int

const (
	ListFilesCached ListFilesMode = iota
	ListFilesDeleted
	ListFilesModified
	ListFilesOthers
	//ListFilesIgnored
	ListFilesStage
)

type LsFilesOptions struct {
	Mode  ListFilesMode
	Z     bool
	JSON  bool
	Paths []string
}

func (opts *LsFilesOptions) newLine() byte {
	if opts.Z {
		return 0x00
	}
	return '\n'
}

func (w *Worktree) lsFilesCached(opts *LsFilesOptions) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	entries := make([]string, 0, 20)
	newLine := opts.newLine()
	m := NewMatcher(opts.Paths)
	for _, e := range idx.Entries {
		if !m.Match(e.Name) {
			continue
		}
		if opts.JSON {
			entries = append(entries, e.Name)
			continue
		}
		fmt.Fprintf(os.Stdout, "%s%c", e.Name, newLine)
	}
	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(entries)
	}
	return nil
}

func (w *Worktree) lsFilesDeleted(ctx context.Context, opts *LsFilesOptions) error {
	changes, err := w.diffStagingWithWorktree(ctx, false, true)
	if err != nil {
		return err
	}
	newLine := opts.newLine()
	m := NewMatcher(opts.Paths)
	entries := make([]string, 0, 20)
	for _, e := range changes {
		action, err := e.Action()
		if err != nil {
			return err
		}
		if action != merkletrie.Delete {
			continue
		}
		p := nameFromAction(&e)
		if !m.Match(p) {
			continue
		}
		if opts.JSON {
			entries = append(entries, p)
			continue
		}
		fmt.Fprintf(os.Stdout, "%s%c", p, newLine)
	}
	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(entries)
	}
	return nil
}

func (w *Worktree) lsFilesModified(ctx context.Context, opts *LsFilesOptions) error {
	changes, err := w.diffStagingWithWorktree(ctx, false, true)
	if err != nil {
		return err
	}
	newLine := opts.newLine()
	m := NewMatcher(opts.Paths)
	entries := make([]string, 0, 20)
	for _, e := range changes {
		action, err := e.Action()
		if err != nil {
			return err
		}
		if action != merkletrie.Delete && action != merkletrie.Modify {
			continue
		}
		p := nameFromAction(&e)
		if !m.Match(p) {
			continue
		}
		if opts.JSON {
			entries = append(entries, p)
			continue
		}
		fmt.Fprintf(os.Stdout, "%s%c", p, newLine)
	}
	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(entries)
	}
	return nil
}

func (w *Worktree) lsFilesOthers(ctx context.Context, opts *LsFilesOptions) error {
	changes, err := w.diffStagingWithWorktree(ctx, false, false)
	if err != nil {
		return err
	}
	ignored := w.ignoredChanges(changes)
	newLine := opts.newLine()
	m := NewMatcher(opts.Paths)
	entries := make([]string, 0, 20)
	for _, e := range ignored {
		if !m.Match(e) {
			continue
		}
		if opts.JSON {
			entries = append(entries, e)
			continue
		}
		fmt.Fprintf(os.Stdout, "%s%c", e, newLine)
	}
	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(entries)
	}
	return nil
}

type StageItem struct {
	Name  string            `json:"name"`
	Mode  filemode.FileMode `json:"mode"`
	Hash  plumbing.Hash     `json:"hash"`
	Stage index.Stage       `json:"stage"`
}

func (w *Worktree) lsFilesStage(opts *LsFilesOptions) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	entries := make([]*StageItem, 0, 20)
	newLine := opts.newLine()
	m := NewMatcher(opts.Paths)
	for _, e := range idx.Entries {
		if !m.Match(e.Name) {
			continue
		}
		if opts.JSON {
			entries = append(entries, &StageItem{
				Name:  e.Name,
				Mode:  e.Mode,
				Hash:  e.Hash,
				Stage: e.Stage,
			})
			continue
		}
		fmt.Fprintf(os.Stdout, "%s %s %d\t%s%c", e.Mode, e.Hash, e.Stage, e.Name, newLine)
	}
	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(entries)
	}
	return nil
}

func (w *Worktree) LsFiles(ctx context.Context, opts *LsFilesOptions) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	switch opts.Mode {
	case ListFilesCached:
		return w.lsFilesCached(opts)
	case ListFilesDeleted:
		return w.lsFilesDeleted(ctx, opts)
	case ListFilesModified:
		return w.lsFilesModified(ctx, opts)
	case ListFilesOthers:
		return w.lsFilesOthers(ctx, opts)
	//case ListFilesIgnored:
	case ListFilesStage:
		return w.lsFilesStage(opts)
	}
	return nil
}
