// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin
// +build darwin

package zeta

import (
	"bytes"
	"strings"
	"syscall"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

const (
	escapeChars = "*?[]\\"
)

func init() {
	fillSystemInfo = func(e *index.Entry, sys any) {
		if os, ok := sys.(*syscall.Stat_t); ok {
			e.CreatedAt = time.Unix(os.Atimespec.Unix())
			e.Dev = uint32(os.Dev)
			e.Inode = uint32(os.Ino)
			e.GID = os.Gid
			e.UID = os.Uid
		}
	}
}

func isSymlinkWindowsNonAdmin(error) bool {
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
	var newItems merkletrie.Changes
	var res merkletrie.Changes
	rmItems := make(map[string]merkletrie.Change)
	for _, ch := range changes {
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
		// Add
		if ch.From == nil {
			newItems = append(newItems, ch)
			continue
		}
		// Del
		if ch.To == nil {
			rmItems[strings.ToLower(ch.From.String())] = ch
			continue
		}
		res = append(res, ch)
	}
	for _, ch := range newItems {
		name := strings.ToLower(ch.To.String())
		if c, ok := rmItems[name]; ok {
			if !bytes.Equal(c.From.Hash(), ch.To.Hash()) {
				ch.From = c.From
				res = append(res, ch) // rename and modify
			}
			delete(rmItems, name)
			continue
		}
		res = append(res, ch)
	}
	for _, ch := range rmItems {
		res = append(res, ch)
	}
	return res
}
