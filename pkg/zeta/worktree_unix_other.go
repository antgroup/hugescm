// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

//go:build openbsd || dragonfly || solaris

package zeta

import (
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
			e.CreatedAt = time.Unix(os.Atim.Unix())
			e.Dev = uint32(os.Dev)
			e.Inode = uint32(os.Ino)
			e.GID = os.Gid
			e.UID = os.Uid
		}
	}
}

func isSymlinkWindowsNonAdmin(_ error) bool {
	return false
}

// canonicalName returns the canonical form of a filename.
// On OpenBSD, DragonFly, and Solaris, filenames are case-sensitive, so we return the name unchanged.
// This ensures that "File.txt" and "file.txt" are treated as different files.
func canonicalName(name string) string {
	return name
}

// systemCaseEqual compares two filenames using platform-specific case sensitivity.
// On OpenBSD, DragonFly, and Solaris, filenames are case-sensitive, so we use exact string comparison.
// This matches the operating system's filesystem behavior.
func systemCaseEqual(a, b string) bool {
	return a == b
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
				// Skip new files that match ignore rules.
				// However, keep deletions and modifications of ignored files.
				// This design allows users to intentionally track deletions of ignored files,
				// which is consistent with common VCS behavior (e.g., Git's `git add -A`).
				// If you want to skip all changes to ignored files including deletions,
				// consider adding a configuration option to control this behavior.
				if len(ch.From) == 0 {
					continue
				}
			}
		}
		res = append(res, ch)
	}
	return res
}
