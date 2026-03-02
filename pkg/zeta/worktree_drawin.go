// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

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

func isSymlinkWindowsNonAdmin(_ error) bool {
	return false
}

// canonicalName returns the canonical form of a filename.
//
// On macOS (Darwin), the filesystem (HFS+, APFS) is case-insensitive by default.
// This means "File.txt" and "file.txt" are treated as the same file by the OS.
//
// This function converts filenames to lowercase to ensure consistent matching,
// which is essential for:
// - Cross-platform Git repositories (Windows/macOS compatibility)
// - Rename detection
// - File set lookups
//
// Note: While macOS filesystems are case-insensitive, they are case-preserving,
// meaning the original case is stored but ignored for comparisons.
func canonicalName(name string) string {
	return strings.ToLower(name)
}

// systemCaseEqual compares two filenames using platform-specific case sensitivity.
//
// On macOS (Darwin), filenames are case-insensitive, so we use case-insensitive comparison.
// This function uses strings.EqualFold which performs Unicode-aware case folding,
// ensuring correct behavior with international characters.
//
// This matches the operating system's filesystem behavior and should be used
// whenever comparing filenames on macOS.
func systemCaseEqual(a, b string) bool {
	return strings.EqualFold(a, b)
}

// excludeIgnoredChanges filters out ignored file changes and handles rename detection.
//
// This function performs the following operations:
// 1. Filters out ignored files using the .zetaignore rules
// 2. Detects file renames by matching deleted and added files with the same canonical name
//
// On macOS, filenames are case-insensitive (but case-preserving), so canonicalName() is used
// to convert filenames to lowercase for consistent matching. This ensures that "File.txt" and
// "file.txt" are treated as the same file, matching the macOS filesystem behavior (HFS+, APFS).
//
// Rename detection works by:
// - Storing deleted files in rmItems map (key: canonicalName)
// - Matching added files against deleted files using canonicalName
// - If both have the same hash, it's a pure rename (skipped as no net change)
// - If hashes differ, it's a rename+modify operation (both kept)
//
// Note: Unlike Windows, macOS supports POSIX file permissions, so there's no need
// for special handling of file mode changes like in unifyChangeFileMode().
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
			rmItems[canonicalName(ch.From.String())] = ch
			continue
		}
		res = append(res, ch)
	}
	for _, ch := range newItems {
		name := canonicalName(ch.To.String())
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
