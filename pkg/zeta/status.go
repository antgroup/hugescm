// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/antgroup/hugescm/modules/term"
)

// Status represents the current status of a Worktree.
// The key of the map is the path of the file.
type Status map[string]*FileStatus

// File returns the FileStatus for a given path, if the FileStatus doesn't
// exists a new FileStatus is added to the map using the path as key.
func (s Status) File(path string) *FileStatus {
	if _, ok := (s)[path]; !ok {
		s[path] = &FileStatus{Worktree: Untracked, Staging: Untracked}
	}

	return s[path]
}

// IsUntracked checks if file for given path is 'Untracked'
func (s Status) IsUntracked(path string) bool {
	stat, ok := (s)[filepath.ToSlash(path)]
	return ok && stat.Worktree == Untracked
}

// IsAdded checks if file for given path is 'Added'.
func (s Status) IsAdded(path string) bool {
	stat, ok := (s)[filepath.ToSlash(path)]
	return ok && stat.Staging == Added
}

// IsModified checks if file for given path is 'Modified'.
func (s Status) IsModified(path string) bool {
	stat, ok := (s)[filepath.ToSlash(path)]
	return ok && stat.Worktree == Modified
}

// IsDeleted checks if file for given path is 'Deleted'.
func (s Status) IsDeleted(path string) bool {
	stat, ok := (s)[filepath.ToSlash(path)]
	return ok && stat.Worktree == Deleted
}

// IsClean returns true if all the files are in Unmodified status.
func (s Status) IsClean() bool {
	for _, status := range s {
		if status.Worktree != Unmodified || status.Staging != Unmodified {
			return false
		}
	}

	return true
}

func (s Status) String() string {
	buf := bytes.NewBuffer(nil)
	for path, status := range s {
		if status.Staging == Unmodified && status.Worktree == Unmodified {
			continue
		}

		if status.Staging == Renamed {
			path = fmt.Sprintf("%s -> %s", path, status.Extra)
		}

		fmt.Fprintf(buf, "%c%c %s\n", status.Staging, status.Worktree, path)
	}

	return buf.String()
}

// FileStatus contains the status of a file in the worktree
type FileStatus struct {
	// Staging is the status of a file in the staging area
	Staging StatusCode
	// Worktree is the status of a file in the worktree
	Worktree StatusCode
	// Extra contains extra information, such as the previous name in a rename
	Extra string
}

// StatusCode status code of a file in the Worktree
type StatusCode byte

const (
	Unmodified         StatusCode = ' '
	Untracked          StatusCode = '?'
	Modified           StatusCode = 'M'
	Added              StatusCode = 'A'
	Deleted            StatusCode = 'D'
	Renamed            StatusCode = 'R'
	Copied             StatusCode = 'C'
	UpdatedButUnmerged StatusCode = 'U'
)

type change struct {
	path string
	*FileStatus
}

type changeOrder []change

func (c changeOrder) Len() int           { return len(c) }
func (c changeOrder) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c changeOrder) Less(i, j int) bool { return c[i].path < c[j].path }

type changes struct {
	Untracked []change
	Staging   []change
	Unstaging []change
	root      string
	cwd       string
}

func newChanges(status Status, root string) *changes {
	cwd, _ := os.Getwd()
	cs := &changes{
		Untracked: make([]change, 0, 20),
		Staging:   make([]change, 0, 20),
		Unstaging: make([]change, 0, 20),
		root:      root,
		cwd:       cwd,
	}
	for p, s := range status {
		if s.Worktree == Unmodified && s.Staging == Unmodified {
			continue
		}
		if s.Worktree == Untracked && s.Staging == Untracked {
			cs.Untracked = append(cs.Untracked, change{path: p, FileStatus: s})
			continue
		}
		if s.Staging != Unmodified {
			cs.Staging = append(cs.Staging, change{path: p, FileStatus: s})
		}
		if s.Worktree != Unmodified {
			cs.Unstaging = append(cs.Unstaging, change{path: p, FileStatus: s})
		}
	}
	sort.Sort(changeOrder(cs.Untracked))
	sort.Sort(changeOrder(cs.Staging))
	sort.Sort(changeOrder(cs.Unstaging))
	return cs
}

func (cs *changes) hasStagedChanges(autoStage bool) bool {
	if autoStage {
		return len(cs.Staging) != 0 || len(cs.Unstaging) != 0
	}
	return len(cs.Staging) != 0
}

func (cs *changes) makePath(name string) string {
	if len(cs.cwd) == 0 {
		return name
	}
	rel, err := filepath.Rel(cs.cwd, filepath.Join(cs.root, name))
	if err != nil {
		return name
	}
	return rel
}

func (cs *changes) show() {
	if len(cs.Staging) != 0 {
		fmt.Fprintf(os.Stdout, "%s\n", W("Changes to be committed:"))
		fmt.Fprintf(os.Stdout, "  %s\n", W("(use \"zeta restore --staged <file>...\" to unstage)"))
		for _, c := range cs.Staging {
			term.Fprintf(os.Stdout, "      \x1b[32m%s\t%s\x1b[0m\n", W(StatusName(c.Staging)), cs.makePath(c.path))
		}

	}
	if len(cs.Unstaging) != 0 {
		fmt.Fprintf(os.Stdout, "%s:\n", W("Changes not staged for commit"))
		fmt.Fprintf(os.Stdout, "  %s\n", W("(use \"zeta add <file>...\" to update what will be committed)"))
		fmt.Fprintf(os.Stdout, "  %s\n", W("(use \"zeta restore <file>...\" to discard changes in working directory)"))
		for _, c := range cs.Unstaging {
			term.Fprintf(os.Stdout, "      \x1b[31m%s\t%s\x1b[0m\n", W(StatusName(c.Worktree)), cs.makePath(c.path))
		}

	}
	if len(cs.Untracked) != 0 {
		fmt.Fprintf(os.Stdout, "%s:\n", W("Untracked files"))
		fmt.Fprintf(os.Stdout, "  %s\n", W("(use \"zeta add <file>...\" to include in what will be committed)"))
		for _, c := range cs.Untracked {
			term.Fprintf(os.Stdout, "      \x1b[31m%s\x1b[0m\n", cs.makePath(c.path))
		}
	}
	if len(cs.Staging) == 0 && len(cs.Unstaging) == 0 {
		fmt.Fprintf(os.Stdout, "%s\n", W("nothing added to commit but untracked files present (use \"zeta add\" to track)"))
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", W("no changes added to commit (use \"zeta add\" and/or \"zeta commit -a\")"))
}

const (
	NUL = '\x00'
)

func statusShow(status Status, root string, z bool) {
	cwd, _ := os.Getwd()
	makePath := func(name string) string {
		if len(cwd) == 0 {
			return name
		}
		rel, err := filepath.Rel(cwd, filepath.Join(root, name))
		if err != nil {
			return name
		}
		return rel
	}
	changes := make([]change, 0, len(status))
	untracked := make([]change, 0, 4)

	for p, s := range status {
		if s.Worktree == Added {
			untracked = append(untracked, change{path: p, FileStatus: s})
			continue
		}
		changes = append(changes, change{path: p, FileStatus: s})
	}
	sort.Sort(changeOrder(changes))
	if !z {
		for _, c := range changes {
			term.Fprintf(os.Stdout, "\x1b[32m%c\x1b[31m%c\x1b[0m %s\n", c.Staging, c.Worktree, makePath(c.path))
		}
		sort.Sort(changeOrder(untracked))
		for _, c := range untracked {
			term.Fprintf(os.Stdout, "\x1b[31m%c%c\x1b[0m %s\n", c.Staging, c.Worktree, makePath(c.path))
		}
		return
	}
	for _, c := range changes {
		fmt.Fprintf(os.Stdout, "%c%c %s%c", c.Staging, c.Worktree, makePath(c.path), NUL)
	}
	sort.Sort(changeOrder(untracked))
	for _, c := range untracked {
		fmt.Fprintf(os.Stdout, "%c%c %s%c", c.Staging, c.Worktree, makePath(c.path), NUL)
	}

}
