// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package zeta

import (
	"os"
	"syscall"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/filesystem"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

const (
	escapeChars = "*?[]"
)

func init() {
	fillSystemInfo = func(e *index.Entry, sys any) {
		if os, ok := sys.(*syscall.Win32FileAttributeData); ok {
			seconds := os.CreationTime.Nanoseconds() / 1000000000
			nanoseconds := os.CreationTime.Nanoseconds() - seconds*1000000000
			e.CreatedAt = time.Unix(seconds, nanoseconds)
		}
	}
}

func isSymlinkWindowsNonAdmin(err error) bool {
	const ERROR_PRIVILEGE_NOT_HELD syscall.Errno = 1314

	if err != nil {
		if errLink, ok := err.(*os.LinkError); ok {
			if errNo, ok := errLink.Err.(syscall.Errno); ok {
				return errNo == ERROR_PRIVILEGE_NOT_HELD
			}
		}
	}

	return false
}

type hasher interface {
	HashRaw() plumbing.Hash
	Mode() filemode.FileMode
}

func (w *Worktree) hijackChange(ch *merkletrie.Change) bool {
	if ch.From == nil || ch.To == nil {
		return false
	}
	from := ch.From.Last()
	to := ch.To.Last()
	a, ok := from.(hasher)
	if !ok {
		return false
	}
	b, ok := to.(hasher)
	if !ok {
		return false
	}
	modeA := a.Mode()
	modeB := b.Mode()
	if a.HashRaw() == b.HashRaw() {
		return modeA&filemode.Regular == modeB&filemode.Regular
	}
	if modeA&filemode.Regular == modeB&filemode.Regular {
		// rewrite change
		if fa, ok := from.(*filesystem.Node); ok {
			fa.HijackMode(modeB)
			return false
		}
		if fb, ok := to.(*filesystem.Node); ok {
			fb.HijackMode(modeA)
		}
	}
	return false
}

func (w *Worktree) excludeIgnoredChanges(changes merkletrie.Changes) merkletrie.Changes {
	if len(changes) == 0 {
		return changes
	}
	m, err := w.ignoreMatcher()
	if err != nil {
		return changes
	}

	var res merkletrie.Changes
	for _, ch := range changes {
		if w.hijackChange(&ch) {
			continue
		}
		var path []string
		for _, n := range ch.To {
			path = append(path, n.Name())
		}
		if len(path) == 0 {
			for _, n := range ch.From {
				path = append(path, n.Name())
			}
		}
		if len(path) != 0 {
			isDir := (len(ch.To) > 0 && ch.To.IsDir()) || (len(ch.From) > 0 && ch.From.IsDir())
			if m.Match(path, isDir) {
				if len(ch.From) == 0 {
					continue
				}
			}
		}
		res = append(res, ch)
	}
	return res
}
