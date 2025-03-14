// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/fnmatch"
	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/filesystem"
	mindex "github.com/antgroup/hugescm/modules/merkletrie/index"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/ignore"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/vfs"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

var (
	statusNameMap = map[StatusCode]string{
		Added:              "new file:",
		Copied:             "copied:",
		Deleted:            "deleted:",
		Modified:           "modified:",
		Renamed:            "renamed:",
		UpdatedButUnmerged: "unmerged:",
	}
)

func StatusName(c StatusCode) string {
	if s, ok := statusNameMap[c]; ok {
		return s
	}
	return "unknown:"
}

var (
	// ErrDestinationExists in an Move operation means that the target exists on
	// the worktree.
	ErrDestinationExists = errors.New("destination exists")
	// ErrGlobNoMatches in an AddGlob if the glob pattern does not match any
	// files in the worktree.
	ErrGlobNoMatches = errors.New("glob pattern did not match any files")
)

func (w *Worktree) ShowFs(verbose bool) {
	if !verbose {
		return
	}
	ds, err := strengthen.GetDiskFreeSpaceEx(w.baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetDiskFreeSpaceEx error: %v\n", err)
		return
	}
	gb := float64(1024 * 1024 * 1024)
	if warningFs[strings.ToLower(ds.FS)] {
		fsName := ds.FS
		if term.StderrLevel != term.LevelNone {
			fsName = "\x1b[01;33m" + ds.FS + "\x1b[0m"
		}
		warn("The repository filesystem is '%s', which may affect zeta's operation.", fsName)
	}
	term.Fprintf(os.Stderr, "\x1b[33m* Worktree filesystem: \x1b[38;2;0;191;255m%s\x1b[0m \x1b[33mused: \x1b[38;2;255;215;0m%0.2f GB\x1b[0m \x1b[33mavail: \x1b[38;2;63;247;166m%0.2f GB\x1b[0m \x1b[33mtotal: \x1b[38;02;39;199;173m%0.2f GB\x1b[0m \n",
		ds.FS, float64(ds.Used)/gb, float64(ds.Avail)/gb, float64(ds.Total)/gb)
}

// Status returns the working tree status.
func (w *Worktree) Status(ctx context.Context, verbose bool) (Status, error) {
	var hash plumbing.Hash

	ref, err := w.Current()
	if err != nil && err != plumbing.ErrReferenceNotFound {
		return nil, err
	}

	if err == nil {
		hash = ref.Hash()
		if verbose {
			if ref.Name().IsBranch() {
				fmt.Fprintf(os.Stderr, "%s %s\n", W("On branch"), ref.Name().BranchName())
			} else {
				fmt.Fprintf(os.Stderr, "%s %s\n", W("HEAD detached at"), ref.Hash())
			}
		}
	}

	return w.status(ctx, hash)
}

func (w *Worktree) status(ctx context.Context, commit plumbing.Hash) (Status, error) {
	s := make(Status)

	left, err := w.diffCommitWithStaging(ctx, commit, false)
	if err != nil {
		return nil, err
	}

	for _, ch := range left {
		a, err := ch.Action()
		if err != nil {
			return nil, err
		}

		fs := s.File(nameFromAction(&ch))
		fs.Worktree = Unmodified

		switch a {
		case merkletrie.Delete:
			s.File(ch.From.String()).Staging = Deleted
		case merkletrie.Insert:
			s.File(ch.To.String()).Staging = Added
		case merkletrie.Modify:
			s.File(ch.To.String()).Staging = Modified
		}
	}

	right, err := w.diffStagingWithWorktree(ctx, false, true)
	if err != nil {
		return nil, err
	}

	for _, ch := range right {
		a, err := ch.Action()
		if err != nil {
			return nil, err
		}

		fs := s.File(nameFromAction(&ch))
		if fs.Staging == Untracked {
			fs.Staging = Unmodified
		}

		switch a {
		case merkletrie.Delete:
			fs.Worktree = Deleted
		case merkletrie.Insert:
			fs.Worktree = Untracked
			fs.Staging = Untracked
		case merkletrie.Modify:
			fs.Worktree = Modified
		}
	}

	return s, nil
}

func nameFromAction(ch *merkletrie.Change) string {
	name := ch.To.String()
	if name == "" {
		return ch.From.String()
	}

	return name
}

func (w *Worktree) diffCommitWithStaging(ctx context.Context, commit plumbing.Hash, reverse bool) (merkletrie.Changes, error) {
	var t *object.Tree
	if !commit.IsZero() {
		c, err := w.odb.Commit(ctx, commit)
		if err != nil {
			return nil, err
		}

		t, err = w.odb.Tree(ctx, c.Tree)
		if err != nil {
			return nil, err
		}
	}

	return w.diffTreeWithStaging(ctx, t, reverse)
}

func (w *Worktree) diffTreeWithStaging(ctx context.Context, t *object.Tree, reverse bool) (merkletrie.Changes, error) {
	var from noder.Noder
	if t != nil {
		from = object.NewTreeRootNode(t, noder.NewSparseTreeMatcher(w.Core.SparseDirs), true)
	}

	idx, err := w.odb.Index()
	if err != nil {
		return nil, err
	}

	to := mindex.NewRootNode(ctx, idx, w.resolveFragmentsIndex)

	if reverse {
		return merkletrie.DiffTreeContext(ctx, to, from, diffTreeIsEquals)
	}

	return merkletrie.DiffTreeContext(ctx, from, to, diffTreeIsEquals)
}

func (w *Worktree) diffTreeWithWorktree(ctx context.Context, t *object.Tree, reverse bool) (merkletrie.Changes, error) {
	var from noder.Noder
	if t != nil {
		from = object.NewTreeRootNode(t, noder.NewSparseTreeMatcher(w.Core.SparseDirs), true)
	}

	to := filesystem.NewRootNode(w.baseDir, noder.NewSparseTreeMatcher(w.Core.SparseDirs))

	if reverse {
		return merkletrie.DiffTreeContext(ctx, to, from, diffTreeIsEquals)
	}

	return merkletrie.DiffTreeContext(ctx, from, to, diffTreeIsEquals)
}

var emptyNoderHash = make([]byte, plumbing.HASH_DIGEST_SIZE+4)

// diffTreeIsEquals is a implementation of noder.Equals, used to compare
// noder.Noder, it compare the content and the length of the hashes.
//
// Since some of the noder.Noder implementations doesn't compute a hash for
// some directories, if any of the hashes is a 36-byte slice of zero values
// the comparison is not done and the hashes are take as different.
func diffTreeIsEquals(a, b noder.Hasher) bool {
	hashA := a.Hash()
	hashB := b.Hash()

	if bytes.Equal(hashA, emptyNoderHash) || bytes.Equal(hashB, emptyNoderHash) {
		return false
	}

	return bytes.Equal(hashA, hashB)
}

func (w *Worktree) resolveFragmentsIndex(ctx context.Context, e *index.Entry) *index.Entry {
	if ff, err := w.odb.Fragments(ctx, e.Hash); err == nil {
		return &index.Entry{
			Hash:         ff.Origin,
			Name:         e.Name,
			CreatedAt:    e.CreatedAt,
			ModifiedAt:   e.ModifiedAt,
			Dev:          e.Dev,
			Inode:        e.Inode,
			Mode:         e.Mode.Origin(),
			UID:          e.UID,
			GID:          e.GID,
			Size:         ff.Size,
			Stage:        e.Stage,
			SkipWorktree: e.SkipWorktree,
			IntentToAdd:  e.IntentToAdd,
		}
	}
	return e
}

func (w *Worktree) diffStagingWithWorktree(ctx context.Context, reverse, excludeIgnoredChanges bool) (merkletrie.Changes, error) {
	idx, err := w.odb.Index()
	if err != nil {
		return nil, err
	}
	from := mindex.NewRootNode(ctx, idx, w.resolveFragmentsIndex)

	to := filesystem.NewRootNode(w.baseDir, noder.NewSparseTreeMatcher(w.Core.SparseDirs))

	var c merkletrie.Changes
	if reverse {
		c, err = merkletrie.DiffTreeContext(ctx, to, from, diffTreeIsEquals)
	} else {
		c, err = merkletrie.DiffTreeContext(ctx, from, to, diffTreeIsEquals)
	}

	if err != nil {
		return nil, err
	}

	if excludeIgnoredChanges {
		return w.excludeIgnoredChanges(c), nil
	}
	return c, nil
}

func (w *Worktree) ignoreMatcher() (ignore.Matcher, error) {
	patterns, err := ignore.ReadPatterns(w.fs, nil)
	if err != nil {
		return nil, err
	}
	patterns = append(patterns, w.Excludes...)
	return ignore.NewMatcher(patterns), nil
}

func (w *Worktree) ignoredChanges(changes merkletrie.Changes) []string {
	m, err := w.ignoreMatcher()
	if err != nil {
		return nil
	}
	ignored := make([]string, 0, 10)
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
					ignored = append(ignored, nameFromAction(&ch))
					continue
				}
			}
		}
	}
	return ignored
}

func (w *Worktree) doAddDirectory(ctx context.Context, idx *index.Index, s Status, directory string, ignorePattern []ignore.Pattern, dryRun bool) (added bool, err error) {
	if len(ignorePattern) > 0 {
		m := ignore.NewMatcher(ignorePattern)
		matchPath := strings.Split(directory, string(os.PathSeparator))
		if m.Match(matchPath, true) {
			// ignore
			return false, nil
		}
	}

	directory = filepath.ToSlash(filepath.Clean(directory))

	for name := range s {
		if !isPathInDirectory(name, directory) {
			continue
		}

		var a bool
		a, _, err = w.doAddFile(ctx, idx, s, name, ignorePattern, dryRun)
		if err != nil {
			return
		}

		added = added || a
	}

	return
}

func isPathInDirectory(path, directory string) bool {
	return directory == "." || strings.HasPrefix(path, directory+"/")
}

// AddWithOptions file contents to the index,  updates the index using the
// current content found in the working tree, to prepare the content staged for
// the next commit.
//
// It typically adds the current content of existing paths as a whole, but with
// some options it can also be used to add content with only part of the changes
// made to the working tree files applied, or remove paths that do not exist in
// the working tree anymore.
func (w *Worktree) AddWithOptions(ctx context.Context, opts *AddOptions) error {
	if err := opts.Validate(w.Repository); err != nil {
		return err
	}

	if opts.All {
		_, err := w.doAdd(ctx, ".", w.Excludes, opts.SkipStatus, opts.DryRun)
		return err
	}

	if opts.Glob != "" {
		return w.AddGlob(ctx, opts.Glob, opts.DryRun)
	}

	_, err := w.doAdd(ctx, opts.Path, make([]ignore.Pattern, 0), opts.SkipStatus, opts.DryRun)
	return err
}

func (w *Worktree) cleanPatterns(paths []string) ([]string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, false, err
	}
	var hasDot bool
	patterns := make([]string, 0, len(paths))
	for _, p := range paths {
		absPath := filepath.Join(cwd, p)
		rel, err := filepath.Rel(w.baseDir, absPath)
		if err != nil {
			return nil, false, err
		}
		slashRel := filepath.ToSlash(rel)
		if hasDotDot(slashRel) {
			return nil, false, fmt.Errorf("fatal: %s: '%s' is outside repository at '%s'", p, p, w.baseDir)
		}
		if slashRel == dot {
			hasDot = true
		}
		patterns = append(patterns, slashRel)
	}
	return patterns, hasDot, nil
}

func (w *Worktree) Add(ctx context.Context, pathSpec []string, dryRun bool) error {
	if len(pathSpec) == 1 && pathSpec[0] == "." {
		return w.AddWithOptions(ctx, &AddOptions{All: true, DryRun: dryRun})
	}
	patterns, hasDot, err := w.cleanPatterns(pathSpec)
	if err != nil {
		return err
	}
	if hasDot {
		return w.AddWithOptions(ctx, &AddOptions{All: true, DryRun: dryRun})
	}
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	status, err := w.Status(ctx, false)
	if err != nil {
		return err
	}
	m := NewMatcher(patterns)
	for p := range status {
		if !m.Match(p) {
			continue
		}
		if _, _, err = w.doAddFile(ctx, idx, status, p, nil, dryRun); err != nil {
			return err
		}
	}
	if dryRun {
		return nil
	}
	return w.odb.SetIndex(idx)
}

func (w *Worktree) AddTracked(ctx context.Context, pathSpec []string, dryRun bool) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	status, err := w.Status(ctx, false)
	if err != nil {
		return err
	}
	m := NewMatcher(pathSpec)
	for p, fs := range status {
		if fs.Worktree != Modified && fs.Worktree != Deleted {
			continue
		}
		if !m.Match(p) {
			continue
		}
		if _, _, err = w.doAddFile(ctx, idx, status, p, nil, dryRun); err != nil {
			return err
		}
	}
	if dryRun {
		return nil
	}
	return w.odb.SetIndex(idx)
}

func (w *Worktree) chmod(ctx context.Context, paths []string, mask bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	for _, p := range paths {
		e, err := idx.Entry(p)
		if err != nil {
			return err
		}
		if mask {
			e.Mode = e.Mode | filemode.Executable
			continue
		}
		e.Mode = e.Mode&^filemode.Executable | filemode.Regular
	}
	return w.odb.SetIndex(idx)
}

func (w *Worktree) Chmod(ctx context.Context, paths []string, mask bool, dryRun bool) error {
	if dryRun {
		return nil
	}
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}
	if err := w.chmod(ctx, paths, mask); err != nil {
		fmt.Fprintf(os.Stderr, "chmod error: %v\n", err)
		return err
	}
	return nil
}

func (w *Worktree) doAdd(ctx context.Context, path string, ignorePattern []ignore.Pattern, skipStatus bool, dryRun bool) (plumbing.Hash, error) {
	idx, err := w.odb.Index()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	var h plumbing.Hash
	var added bool
	fi, err := w.fs.Lstat(path)
	// status is required for doAddDirectory
	var s Status
	var err2 error
	if !skipStatus || fi == nil || fi.IsDir() {
		if s, err2 = w.Status(ctx, false); err2 != nil {
			return plumbing.ZeroHash, err2
		}
	}
	if err != nil {
		return h, err
	}
	if fi.IsDir() {
		added, err = w.doAddDirectory(ctx, idx, s, path, ignorePattern, dryRun)
	} else {
		added, h, err = w.doAddFile(ctx, idx, s, path, ignorePattern, dryRun)
	}
	if err != nil {
		return h, err
	}

	if !added {
		return h, nil
	}
	if dryRun {
		return h, nil
	}
	return h, w.odb.SetIndex(idx)
}

// AddGlob adds all paths, matching pattern, to the index. If pattern matches a
// directory path, all directory contents are added to the index recursively. No
// error is returned if all matching paths are already staged in index.
func (w *Worktree) AddGlob(ctx context.Context, pattern string, dryRun bool) error {
	files, err := vfs.Glob(w.fs, pattern)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return ErrGlobNoMatches
	}

	s, err := w.Status(ctx, false)
	if err != nil {
		return err
	}

	idx, err := w.odb.Index()
	if err != nil {
		return err
	}

	var saveIndex bool
	for _, file := range files {
		fi, err := w.fs.Lstat(file)
		if err != nil {
			return err
		}

		var added bool
		if fi.IsDir() {
			added, err = w.doAddDirectory(ctx, idx, s, file, make([]ignore.Pattern, 0), dryRun)
		} else {
			added, _, err = w.doAddFile(ctx, idx, s, file, make([]ignore.Pattern, 0), dryRun)
		}

		if err != nil {
			return err
		}

		if !saveIndex && added {
			saveIndex = true
		}
	}
	if dryRun {
		return nil
	}

	if saveIndex {
		return w.odb.SetIndex(idx)
	}

	return nil
}

// doAddFile create a new blob from path and update the index, added is true if
// the file added is different from the index.
// if s status is nil will skip the status check and update the index anyway
func (w *Worktree) doAddFile(ctx context.Context, idx *index.Index, s Status, path string, ignorePattern []ignore.Pattern, dryRun bool) (added bool, h plumbing.Hash, err error) {
	if s != nil && s.File(path).Worktree == Unmodified {
		return false, h, nil
	}
	if len(ignorePattern) > 0 {
		m := ignore.NewMatcher(ignorePattern)
		matchPath := strings.Split(path, string(os.PathSeparator))
		if m.Match(matchPath, true) {
			// ignore
			return false, h, nil
		}
	}
	if dryRun {
		fmt.Fprintf(os.Stdout, "add '%s'\n", path)
		return false, h, nil
	}
	if s.IsDeleted(path) {
		added = true
		h, err = w.deleteFromIndex(idx, path)
		return
	}

	w.DbgPrint("add '%s'", path)
	var asFragments bool
	if h, asFragments, err = w.copyFileToStorage(ctx, path); err != nil {
		return
	}

	if err := w.addOrUpdateFileToIndex(idx, path, h, asFragments); err != nil {
		return false, h, err
	}

	return true, h, err
}

func (w *Worktree) copyFileToStorage(ctx context.Context, path string) (plumbing.Hash, bool, error) {
	fi, err := w.fs.Lstat(path)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := w.fs.Readlink(path)
		if err != nil {
			return plumbing.ZeroHash, false, err
		}
		oid, err := w.odb.HashTo(ctx, strings.NewReader(target), int64(len(target)))
		return oid, false, err
	}
	fd, err := w.fs.Open(path)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	defer fd.Close()
	return w.HashTo(ctx, fd, fi.Size())
}

func (w *Worktree) addOrUpdateFileToIndex(idx *index.Index, filename string, h plumbing.Hash, asFragments bool) error {
	e, err := idx.Entry(filename)
	if err != nil && err != index.ErrEntryNotFound {
		return err
	}

	if err == index.ErrEntryNotFound {
		return w.doAddFileToIndex(idx, filename, h, asFragments)
	}

	return w.doUpdateFileToIndex(e, filename, h, asFragments)
}

func (w *Worktree) doAddFileToIndex(idx *index.Index, filename string, h plumbing.Hash, asFragments bool) error {
	return w.doUpdateFileToIndex(idx.Add(filename), filename, h, asFragments)
}

func (w *Worktree) doUpdateFileToIndex(e *index.Entry, filename string, h plumbing.Hash, asFragments bool) error {
	info, err := w.fs.Lstat(filename)
	if err != nil {
		return err
	}

	e.Size = uint64(info.Size())
	e.Hash = h
	e.ModifiedAt = info.ModTime()
	e.Mode, err = filemode.NewFromOS(info.Mode())
	if err != nil {
		return err
	}
	// check object is fragments
	if asFragments {
		e.Mode |= filemode.Fragments
	}

	fillSystemInfo(e, info.Sys())
	return nil
}

// RemoveLegacy removes files from the working tree and from the index.
func (w *Worktree) RemoveLegacy(path string) (plumbing.Hash, error) {
	// TODO(mcuadros): remove plumbing.Hash from signature at v5.
	idx, err := w.odb.Index()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	var h plumbing.Hash

	fi, err := w.fs.Lstat(path)
	if err != nil || !fi.IsDir() {
		h, err = w.doRemoveFile(idx, path)
	} else {
		_, err = w.doRemoveDirectory(idx, path)
	}
	if err != nil {
		return h, err
	}

	return h, w.odb.SetIndex(idx)
}

func (w *Worktree) doRemoveDirectory(idx *index.Index, directory string) (removed bool, err error) {
	entries, err := w.fs.ReadDir(directory)
	if err != nil {
		return false, err
	}

	for _, file := range entries {
		name := path.Join(directory, file.Name())

		var r bool
		if file.IsDir() {
			r, err = w.doRemoveDirectory(idx, name)
		} else {
			_, err = w.doRemoveFile(idx, name)
			if err == index.ErrEntryNotFound {
				err = nil
			}
		}

		if err != nil {
			return
		}

		if !removed && r {
			removed = true
		}
	}

	err = w.removeEmptyDirectory(directory)
	return
}

func (w *Worktree) removeEmptyDirectory(path string) error {
	entries, err := w.fs.ReadDir(path)
	if err != nil {
		return err
	}

	if len(entries) != 0 {
		return nil
	}

	return w.fs.Remove(path)
}

func (w *Worktree) doRemoveFile(idx *index.Index, path string) (plumbing.Hash, error) {
	hash, err := w.deleteFromIndex(idx, path)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return hash, w.deleteFromFilesystem(path)
}

func (w *Worktree) deleteFromIndex(idx *index.Index, path string) (plumbing.Hash, error) {
	e, err := idx.Remove(path)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return e.Hash, nil
}

func (w *Worktree) deleteFromFilesystem(path string) error {
	err := w.fs.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// RemoveGlob removes all paths, matching pattern, from the index. If pattern
// matches a directory path, all directory contents are removed from the index
// recursively.
func (w *Worktree) RemoveGlob(pattern string) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}

	entries, err := idx.Glob(pattern)
	if err != nil {
		return err
	}

	for _, e := range entries {
		file := filepath.FromSlash(e.Name)
		if _, err := w.fs.Lstat(file); err != nil && !os.IsNotExist(err) {
			return err
		}

		if _, err := w.doRemoveFile(idx, file); err != nil {
			return err
		}

		dir, _ := filepath.Split(file)
		if err := w.removeEmptyDirectory(dir); err != nil {
			return err
		}
	}

	return w.odb.SetIndex(idx)
}

type RemoveOptions struct {
	DryRun  bool
	Cached  bool
	Force   bool
	Recurse bool
}

func matchEx(idx *index.Index, pattern string, recurse bool) (matches []*index.Entry, err error) {
	if strings.ContainsAny(pattern, escapeChars) {
		for _, e := range idx.Entries {
			if fnmatch.Match(pattern, e.Name, 0) {
				matches = append(matches, e)
			}
		}
		return
	}
	pattern = filepath.ToSlash(strings.TrimSuffix(pattern, "/"))
	prefixLen := len(pattern)
	for _, e := range idx.Entries {
		if len(e.Name) < prefixLen || !systemCaseEqual(e.Name[0:prefixLen], pattern) {
			continue
		}
		if len(e.Name) == prefixLen {
			matches = append(matches, e)
			continue
		}
		if e.Name[prefixLen] == '/' {
			if !recurse {
				return nil, fmt.Errorf("not removing '%s' recursively without -r", pattern)
			}
			matches = append(matches, e)
		}
	}
	return
}

func (w *Worktree) Remove(ctx context.Context, patterns []string, opts *RemoveOptions) error {
	if len(patterns) == 0 {
		return nil
	}
	status, err := w.Status(ctx, false)
	if err != nil {
		return err
	}
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	entries := make([]*index.Entry, 0, 10)
	for _, pattern := range patterns {
		subEntries, err := matchEx(idx, pattern, opts.Recurse)
		if err != nil {
			return err
		}
		if len(subEntries) == 0 {
			return fmt.Errorf("pathspec '%s' did not match any files", pattern)
		}
		entries = append(entries, subEntries...)
	}
	seen := make(map[string]bool)
	for _, e := range entries {
		if seen[e.Name] {
			continue
		}
		seen[e.Name] = true
		file := filepath.FromSlash(e.Name)
		if _, err := w.fs.Lstat(file); err != nil && !os.IsNotExist(err) {
			return err
		}
		if opts.Cached {
			fmt.Fprintf(os.Stderr, "rm '%s'\n", e.Name)
			if opts.DryRun {
				continue
			}
			if _, err := w.deleteFromIndex(idx, file); err != nil {
				return err
			}
			continue
		}
		if s, ok := status[e.Name]; ok {
			if s.Worktree == Modified && !opts.Force {
				return fmt.Errorf("'%s' has local modifications\n(use --cached to keep the file, or -f to force removal)", e.Name)
			}
		}
		fmt.Fprintf(os.Stderr, "rm '%s'\n", e.Name)
		if opts.DryRun {
			continue
		}
		if _, err := w.doRemoveFile(idx, file); err != nil {
			return err
		}
		dir, _ := filepath.Split(file)
		if err := w.removeEmptyDirectory(dir); err != nil {
			return err
		}
	}
	return w.odb.SetIndex(idx)
}

func (w *Worktree) ShowStatus(status Status, short bool, z bool) {
	if short {
		statusShow(status, w.baseDir, z)
		return
	}
	newChanges(status, w.baseDir).show()
}

/*
0
One or more of the provided paths is ignored.

1
None of the provided paths are ignored.

128
A fatal error was encountered.
*/

type CheckIgnoreOption struct {
	Paths []string
	Stdin bool
	Z     bool
	JSON  bool
}

func (opts *CheckIgnoreOption) newLine() byte {
	if opts.Z {
		return 0x00
	}
	return '\n'
}

func (opts *CheckIgnoreOption) paths() ([]string, error) {
	if !opts.Stdin {
		return opts.Paths, nil
	}
	br := bufio.NewReader(os.Stdin)
	paths := make([]string, 0, 10)
	newLine := opts.newLine()
	for {
		s, readErr := br.ReadString(newLine)
		if readErr != nil && readErr != io.EOF {
			return nil, readErr
		}
		line := strings.TrimRightFunc(s, func(r rune) bool {
			return r == rune(newLine)
		})
		if len(line) != 0 {
			paths = append(paths, line)
		}
		if readErr == io.EOF {
			break
		}
	}
	return paths, nil
}

func (w *Worktree) DoCheckIgnore(ctx context.Context, opts *CheckIgnoreOption) error {
	paths, err := opts.paths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read input paths error: %v\n", err)
		return &ErrExitCode{ExitCode: 128, Message: err.Error()}
	}
	w.DbgPrint("%v", paths)
	m, err := w.ignoreMatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "new ignore matcher error: %v\n", err)
		return &ErrExitCode{ExitCode: 128, Message: err.Error()}
	}
	matched := make([]string, 0, 10)
	for _, p := range paths {
		if m.Match(filepath.SplitList(p), false) {
			matched = append(matched, p)
		}
	}
	if len(matched) == 0 {
		return &ErrExitCode{ExitCode: 1, Message: "none of the provided paths are ignored"}
	}
	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(matched)
	}
	newLine := opts.newLine()
	for _, p := range matched {
		fmt.Fprintf(os.Stdout, "%s%c", p, newLine)
	}
	return nil
}
