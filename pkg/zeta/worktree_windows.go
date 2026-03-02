// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package zeta

import (
	"bytes"
	"os"
	"strings"
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

// canonicalName returns the canonical form of a filename.
// On Windows, filenames are case-insensitive, so we convert to lowercase.
// This ensures that "File.txt" and "file.txt" are treated as the same file.
func canonicalName(name string) string {
	return strings.ToLower(name)
}

// systemCaseEqual compares two filenames using platform-specific case sensitivity.
// On Windows, filenames are case-insensitive, so we use case-insensitive comparison.
// This matches the operating system's filesystem behavior.
func systemCaseEqual(a, b string) bool {
	return strings.EqualFold(a, b)
}

type hasher interface {
	HashRaw() plumbing.Hash
	Mode() filemode.FileMode
}

// unifyChangeFileMode handles file mode changes on Windows.
//
// Windows does not use the POSIX file permission model (rwx permissions) and instead
// uses ACL (Access Control Lists). However, Zeta still stores POSIX permission modes
// in the index, which can lead to false-positive changes when files are modified on Windows.
//
// This function addresses this issue by:
// 1. Skipping changes where only the file mode changed but the content is identical
// 2. Unifying file modes when the content actually changed to eliminate permission noise
//
// The function only considers the filemode.Regular flag, which distinguishes between
// regular files and other file types (directories, symlinks, etc.). The executable bit
// and other POSIX-specific permissions are ignored on Windows.
//
// Returns true if the change should be skipped (false positive), false otherwise.
func (w *Worktree) unifyChangeFileMode(ch *merkletrie.Change) bool {
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

	// Case 1: Content is identical (same hash)
	// Only the Regular flag matters - if it's the same, skip this change
	if a.HashRaw() == b.HashRaw() {
		return modeA&filemode.Regular == modeB&filemode.Regular
	}

	// Case 2: Regular flag is the same but content changed
	// Unify the file modes to eliminate permission noise, but keep the content change
	if modeA&filemode.Regular == modeB&filemode.Regular {
		// Rewrite the change by unifying file modes to eliminate permission noise
		if fa, ok := from.(*filesystem.Node); ok {
			fa.UnifyMode(modeB)
			return false
		}
		if fb, ok := to.(*filesystem.Node); ok {
			fb.UnifyMode(modeA)
		}
	}
	return false
}

// excludeIgnoredChanges filters out ignored file changes and handles rename detection.
//
// This function performs the following operations:
// 1. Filters out ignored files using the .zetaignore rules
// 2. Detects file renames by matching deleted and added files with the same canonical name
// 3. On Windows, calls unifyChangeFileMode to ignore meaningless POSIX permission changes
//
// On Windows, filenames are case-insensitive, so canonicalName() is used to convert
// filenames to lowercase for consistent matching. This ensures that "File.txt" and "file.txt"
// are treated as the same file, matching the Windows filesystem behavior.
//
// Rename detection works by:
// - Storing deleted files in rmItems map (key: canonicalName)
// - Matching added files against deleted files using canonicalName
// - If both have the same hash, it's a pure rename (skipped as no net change)
// - If hashes differ, it's a rename+modify operation (both kept)
//
// Parameters:
//
//	changes: List of file changes to process
//
// Returns:
//
//	Filtered list of changes with ignored files removed and renames detected
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
		// modified
		if w.unifyChangeFileMode(&ch) {
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
