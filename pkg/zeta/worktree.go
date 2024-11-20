// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/ignore"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
	"github.com/antgroup/hugescm/modules/vfs"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/progress"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

var (
	ErrWorktreeNotEmpty     = errors.New("worktree not empty")
	ErrWorktreeNotClean     = errors.New("worktree is not clean")
	ErrUnstagedChanges      = errors.New("worktree contains unstaged changes")
	ErrNonFastForwardUpdate = errors.New("non-fast-forward update")
)

// Worktree represents a zeta worktree.
type Worktree struct {
	// External excludes not found in the repository .gitignore/.zetaignore
	Excludes []ignore.Pattern
	fs       vfs.VFS
	*Repository
}

type ProgressBar interface {
	Add(int)
}

type nonProgressBar struct {
}

func (p nonProgressBar) Add(int) {}

var (
	_ ProgressBar = &nonProgressBar{}
)

func (w *Worktree) createBranch(opts *CheckoutOptions) error {
	_, err := w.Reference(opts.Branch)
	if err == nil {
		return fmt.Errorf("a branch named %q already exists", opts.Branch)
	}

	if err != plumbing.ErrReferenceNotFound {
		return err
	}

	if opts.Hash.IsZero() {
		ref, err := w.Current()
		if err != nil {
			return err
		}

		opts.Hash = ref.Hash()
	}

	return w.ReferenceUpdate(plumbing.NewHashReference(opts.Branch, opts.Hash), nil)
}

func (w *Worktree) getCommitFromCheckoutOptions(ctx context.Context, opts *CheckoutOptions) (plumbing.Hash, error) {
	if !opts.Hash.IsZero() {
		return opts.Hash, nil
	}

	b, err := w.Reference(opts.Branch)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	if !b.Name().IsTag() {
		return b.Hash(), nil
	}

	o, err := w.odb.ParseRevExhaustive(ctx, b.Hash())
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return o.Hash, nil
}

func (w *Worktree) containsUnstagedChanges(ctx context.Context) (bool, error) {
	ch, err := w.diffStagingWithWorktree(ctx, false, true)
	if err != nil {
		return false, err
	}

	for _, c := range ch {
		a, err := c.Action()
		if err != nil {
			return false, err
		}

		if a == merkletrie.Insert {
			continue
		}

		return true, nil
	}

	return false, nil
}

func (w *Worktree) setHEADToCommit(commit plumbing.Hash) error {
	originHEAD, err := w.HEAD()
	if err != nil {
		return err
	}
	var from string
	switch {
	case originHEAD == nil:
	case originHEAD.Type() == plumbing.HashReference:
		from = originHEAD.Hash().String()
	default:
		from = originHEAD.Name().Short()
	}
	newHEAD := plumbing.NewHashReference(plumbing.HEAD, commit)
	if err := w.ReferenceUpdate(newHEAD, originHEAD); err != nil {
		return err
	}
	return w.writeHEADReflog(commit, w.NewCommitter(), fmt.Sprintf("switch: move %s from to %s", from, commit))
}

func (w *Worktree) setHEADToBranch(branch plumbing.ReferenceName, commit plumbing.Hash) error {
	originHEAD, err := w.HEAD()
	if err != nil {
		return err
	}

	var from string
	switch {
	case originHEAD == nil:
	case originHEAD.Type() == plumbing.HashReference:
		from = originHEAD.Hash().String()
	default:
		from = originHEAD.Name().Short()
	}

	target, err := w.Reference(branch)
	if err != nil {
		return err
	}

	var head *plumbing.Reference
	if target.Name().IsBranch() {
		head = plumbing.NewSymbolicReference(plumbing.HEAD, target.Name())
	} else {
		head = plumbing.NewHashReference(plumbing.HEAD, commit)
	}

	if err := w.ReferenceUpdate(head, originHEAD); err != nil {
		return err
	}
	return w.writeHEADReflog(commit, w.NewCommitter(), fmt.Sprintf("switch: move %s from to %s", from, branch.Short()))
}

// resetHEAD: like zeta reset $commit --hard
func (w *Worktree) resetHEAD(ctx context.Context, commit plumbing.Hash) error {
	originHEAD, err := w.HEAD()
	if err != nil {
		return err
	}
	if originHEAD == nil {
		return errors.New("HEAD not found")
	}

	if originHEAD.Type() == plumbing.HashReference {
		if err := w.ReferenceUpdate(plumbing.NewHashReference(plumbing.HEAD, commit), originHEAD); err != nil {
			return err
		}
		return nil
	}

	current, err := w.Reference(originHEAD.Target())
	if err != nil {
		return err
	}
	refname := current.Name()
	if !refname.IsBranch() {
		return fmt.Errorf("invalid HEAD target should be a branch, found %s", current.Type())
	}
	message := fmt.Sprintf("reset: move %s from %s to %s", refname.BranchName(), current.Hash(), commit)
	if err := w.DoUpdate(ctx, refname, current.Hash(), commit, w.NewCommitter(), message); err != nil {
		return err
	}
	return nil
}

func (r *Repository) getTreeFromCommitHash(ctx context.Context, commit plumbing.Hash) (*object.Tree, error) {
	c, err := r.odb.ParseRevExhaustive(ctx, commit)
	if err != nil {
		return nil, err
	}
	tree, err := r.odb.Tree(ctx, c.Tree)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

func (r *Repository) getTreeFromHash(ctx context.Context, oid plumbing.Hash) (*object.Tree, error) {
	o, err := r.odb.Object(ctx, oid)
	if err != nil {
		return nil, err
	}
	if tree, ok := o.(*object.Tree); ok {
		return tree, nil
	}
	if cc, ok := o.(*object.Commit); ok {
		return r.getTreeFromHash(ctx, cc.Tree)
	}
	return nil, fmt.Errorf("object '%s' not tree or commit", oid)
}

func (w *Worktree) Reset(ctx context.Context, opts *ResetOptions) error {
	if opts.One {
		w.missingNotFailure = true
	}
	bar := progress.NewIndicators("Checkout files", "Checkout files completed", 0, opts.Quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	if err := w.ResetSparsely(ctx, opts, bar); err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	if opts.Mode != HardReset {
		return nil
	}
	if opts.One {
		if err := w.checkoutOneAfterAnother(ctx); err != nil {
			return err
		}
	}
	current, err := w.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve HEAD error: %v\n", err)
		return err
	}
	cc, err := w.odb.ParseRevExhaustive(ctx, current.Hash())
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve HEAD commit error: %v\n", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "HEAD %s %s %s\n", W("is now at"), shortHash(cc.Hash), cc.Subject())
	return nil
}

func (w *Worktree) resetMixedChanges(changes merkletrie.Changes) {
	fmt.Fprintln(os.Stderr, W("Unstaged changes after reset:"))
	for _, c := range changes {
		action, err := c.Action()
		if err != nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "%c    %s\n", action.Byte(), nameFromAction(&c))
	}
}

func (w *Worktree) ResetSparsely(ctx context.Context, opts *ResetOptions, bar ProgressBar) error {
	if bar == nil {
		bar = &nonProgressBar{}
	}
	if err := opts.Validate(w.Repository); err != nil {
		return err
	}
	switch opts.Mode {
	case MergeReset:
		// FIXME try merge
		unstaged, err := w.containsUnstagedChanges(ctx)
		if err != nil {
			return err
		}

		if unstaged {
			return ErrUnstagedChanges
		}
	case MixedReset:
		changes, err := w.unstagedChanges(ctx)
		if err != nil {
			return err
		}
		if len(changes) != 0 {
			w.resetMixedChanges(changes)
			return nil
		}
	default:
	}

	if err := w.resetHEAD(ctx, opts.Commit); err != nil {
		return err
	}

	if opts.Mode == SoftReset {
		return nil
	}

	t, err := w.getTreeFromCommitHash(ctx, opts.Commit)
	if err != nil {
		return err
	}

	if opts.Mode == MixedReset || opts.Mode == MergeReset || opts.Mode == HardReset {
		if err := w.resetIndex(ctx, t); err != nil {
			return err
		}
	}
	if opts.Mode == MergeReset || opts.Mode == HardReset {
		if err := w.resetWorktree(ctx, t, bar); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worktree) ResetSpec(ctx context.Context, oid plumbing.Hash, pathSpec []string) error {
	root, err := w.getTreeFromHash(ctx, oid)
	if err != nil {
		return err
	}
	m := NewMatcher(pathSpec)
	entries, err := w.lsTreeRecurseFilter(ctx, root, m)
	if err != nil {
		return err
	}
	if err := w.resetIndexMatch(ctx, entries); err != nil {
		return err
	}
	return nil
}

func (w *Worktree) resetIndex(ctx context.Context, t *object.Tree) error {
	idx, err := w.odb.Index()

	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)

	changes, err := w.diffTreeWithStaging(ctx, t, true)
	if err != nil {
		return err
	}

	for _, ch := range changes {
		a, err := ch.Action()
		if err != nil {
			return err
		}

		var name string
		var e *object.TreeEntry

		switch a {
		case merkletrie.Modify, merkletrie.Insert:
			name = ch.To.String()
			e, err = t.FindEntry(ctx, name)
			if err != nil {
				return err
			}
		case merkletrie.Delete:
			name = ch.From.String()
		}

		b.Remove(name)
		if e == nil {
			continue
		}
		b.Add(&index.Entry{
			Name: name,
			Hash: e.Hash,
			Mode: e.Mode,
			Size: uint64(e.Size),
		})
	}

	b.Write(idx)
	return w.odb.SetIndex(idx)
}

var windowsPathReplacer *strings.Replacer

func init() {
	windowsPathReplacer = strings.NewReplacer(" ", "", ".", "")
}

func windowsValidPath(part string) bool {
	if len(part) > 3 && strings.EqualFold(part[:4], ZetaDirName) {
		// For historical reasons, file names that end in spaces or periods are
		// automatically trimmed. Therefore, `.git . . ./` is a valid way to refer
		// to `.git/`.
		if windowsPathReplacer.Replace(part[4:]) == "" {
			return false
		}

		// For yet other historical reasons, NTFS supports so-called "Alternate Data
		// Streams", i.e. metadata associated with a given file, referred to via
		// `<filename>:<stream-name>:<stream-type>`. There exists a default stream
		// type for directories, allowing `.git/` to be accessed via
		// `.git::$INDEX_ALLOCATION/`.
		//
		// For performance reasons, _all_ Alternate Data Streams of `.git/` are
		// forbidden, not just `::$INDEX_ALLOCATION`.
		if len(part) > 4 && part[4:5] == ":" {
			return false
		}
	}
	return true
}

// worktreeDeny is a list of paths that are not allowed
// to be used when resetting the worktree.
var worktreeDeny = map[string]struct{}{
	// .git
	ZetaDirName: {},
	"zeta~":     {},

	// For other historical reasons, file names that do not conform to the 8.3
	// format (up to eight characters for the basename, three for the file
	// extension, certain characters not allowed such as `+`, etc) are associated
	// with a so-called "short name", at least on the `C:` drive by default.
	// Which means that `git~1/` is a valid way to refer to `.git/`.
	"git~1": {},
}

// validPath checks whether paths are valid.
// The rules around invalid paths could differ from upstream based on how
// filesystems are managed within go-git, but they are largely the same.
//
// For upstream rules:
// https://github.com/git/git/blob/564d0252ca632e0264ed670534a51d18a689ef5d/read-cache.c#L946
// https://github.com/git/git/blob/564d0252ca632e0264ed670534a51d18a689ef5d/path.c#L1383
func validPath(paths ...string) error {
	for _, p := range paths {
		parts := strings.FieldsFunc(p, func(r rune) bool { return (r == '\\' || r == '/') })
		if len(parts) == 0 {
			return fmt.Errorf("invalid path: %q", p)
		}

		if _, denied := worktreeDeny[strings.ToLower(parts[0])]; denied {
			return fmt.Errorf("invalid path prefix: %q", p)
		}

		if runtime.GOOS == "windows" {
			// Volume names are not supported, in both formats: \\ and <DRIVE_LETTER>:.
			if vol := filepath.VolumeName(p); vol != "" {
				return fmt.Errorf("invalid path: %q", p)
			}

			if !windowsValidPath(parts[0]) {
				return fmt.Errorf("invalid path: %q", p)
			}
		}

		for _, part := range parts {
			if part == ".." {
				return fmt.Errorf("invalid path %q: cannot use '..'", p)
			}
		}
	}
	return nil
}

func (w *Worktree) validChange(ch merkletrie.Change) error {
	action, err := ch.Action()
	if err != nil {
		return nil
	}

	switch action {
	case merkletrie.Delete:
		return validPath(ch.From.String())
	case merkletrie.Insert:
		return validPath(ch.To.String())
	case merkletrie.Modify:
		return validPath(ch.From.String(), ch.To.String())
	}

	return nil
}

func (w *Worktree) resetWorktree(ctx context.Context, t *object.Tree, bar ProgressBar) error {
	changes, err := w.diffStagingWithWorktree(ctx, true, false)
	if err != nil {
		return err
	}

	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)

	for _, ch := range changes {
		if err := w.validChange(ch); err != nil {
			return err
		}
		if err := w.checkoutChange(ctx, ch, t, b, bar); err != nil {
			return err
		}
	}

	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) resetIndexMatch(ctx context.Context, entries []*odb.TreeEntry) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	idx, err := w.odb.Index()

	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)
	modifiedAt := time.Now()
	for _, e := range entries {
		b.Add(&index.Entry{
			Name:       e.Path,
			Hash:       e.Hash,
			Mode:       e.Mode,
			Size:       uint64(e.Size),
			ModifiedAt: modifiedAt,
		})
	}
	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) resetWorktreeEntriesWorktreeOnly(ctx context.Context, entries []*odb.TreeEntry, bar ProgressBar) error {
	for _, e := range entries {
		err := w.checkoutFile(ctx, e.Path, e.TreeEntry, bar)
		if plumbing.IsNoSuchObject(err) && w.missingNotFailure {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Worktree) resetWorktreeEntries(ctx context.Context, entries []*odb.TreeEntry, bar ProgressBar) error {
	idx, err := w.odb.Index()
	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)
	for _, e := range entries {
		if err = w.checkoutFile(ctx, e.Path, e.TreeEntry, bar); plumbing.IsNoSuchObject(err) && w.missingNotFailure {
			w.addPseudoIndex(e.Path, e.TreeEntry, b)
			return nil
		}
		if err != nil {
			return err
		}
		if err := w.addIndexFromFile(e.Path, e.Hash, e.Mode, b); err != nil {
			return err
		}
	}
	b.Write(idx)
	return w.odb.SetIndex(idx)
}

func (w *Worktree) checkoutChange(ctx context.Context, ch merkletrie.Change, t *object.Tree, idx *indexBuilder, bar ProgressBar) error {
	a, err := ch.Action()
	if err != nil {
		return err
	}

	var e *object.TreeEntry
	var name string
	switch a {
	case merkletrie.Modify, merkletrie.Insert:
		name = ch.To.String()
		if e, err = t.FindEntry(ctx, name); err != nil {
			return err
		}
	case merkletrie.Delete:
		return rmFileAndDirsIfEmpty(w.fs, ch.From.String())
	default:
		return nil
	}
	return w.checkoutChangeRegularFile(ctx, name, a, e, idx, bar)
}

func (w *Worktree) addPseudoIndex(name string, e *object.TreeEntry, b *indexBuilder) {
	now := time.Now()
	b.Remove(name)
	b.Add(&index.Entry{
		Hash:       e.Hash,
		Name:       name,
		Mode:       e.Mode,
		ModifiedAt: now,
		CreatedAt:  now,
		Size:       uint64(e.Size),
	})
}

func (w *Worktree) checkoutChangeRegularFile(ctx context.Context, name string, a merkletrie.Action, e *object.TreeEntry, idx *indexBuilder, bar ProgressBar) error {
	if len(name) == 0 {
		return nil
	}
	switch a {
	case merkletrie.Modify:
		idx.Remove(name)

		// to apply perm changes the file is deleted, vfs doesn't implement
		// chmod
		if err := w.fs.Remove(name); err != nil {
			return err
		}

		fallthrough
	case merkletrie.Insert:
		var err error
		if err = w.checkoutFile(ctx, name, e, bar); plumbing.IsNoSuchObject(err) && w.missingNotFailure {
			w.addPseudoIndex(name, e, idx)
			return nil
		}
		if err != nil {
			return err
		}
		return w.addIndexFromFile(name, e.Hash, e.Mode, idx)
	}
	return nil
}

func (w *Worktree) checkoutFile(ctx context.Context, name string, e *object.TreeEntry, bar ProgressBar) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	var mode os.FileMode
	if mode, err = e.Mode.ToOSFileMode(); err != nil {
		return err
	}
	if mode&os.ModeSymlink != 0 {
		return w.checkoutSymlink(ctx, name, e)
	}
	var fd *os.File
	if fd, err = w.fs.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode); err != nil {
		return err
	}
	defer func() {
		_ = fd.Close()
		if err != nil {
			_ = w.fs.Remove(name)
		}
	}()
	if len(e.Payload) != 0 {
		if _, err = fd.Write(e.Payload); err != nil {
			return
		}
		bar.Add(1)
		return
	}
	if e.Type() == object.FragmentsObject {
		var ff *object.Fragments
		if ff, err = w.odb.Fragments(ctx, e.Hash); err != nil {
			return
		}
		for _, ee := range ff.Entries {
			if err = w.odb.DecodeTo(ctx, fd, ee.Hash, -1); err != nil {
				return
			}
		}
		bar.Add(1)
		return
	}
	if err = w.odb.DecodeTo(ctx, fd, e.Hash, -1); err != nil {
		return
	}
	bar.Add(1)
	return
}

func (w *Worktree) checkoutSymlink(ctx context.Context, name string, e *object.TreeEntry) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	target := string(e.Payload)
	if len(target) == 0 {
		var b strings.Builder
		if err := w.odb.DecodeTo(ctx, &b, e.Hash, 32*1024); err != nil {
			return err
		}
		target = b.String()
	}
	err = w.fs.Symlink(target, name)
	if err != nil && isSymlinkWindowsNonAdmin(err) {
		mode, _ := e.Mode.ToOSFileMode()
		var to *os.File
		if to, err = w.fs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm()); err != nil {
			return err
		}
		defer to.Close()
		_, err = to.WriteString(target)
		return err
	}
	return
}

func (w *Worktree) addIndexFromFile(name string, h plumbing.Hash, mode filemode.FileMode, idx *indexBuilder) error {
	idx.Remove(name)
	fi, err := w.fs.Lstat(name)
	if err != nil {
		return err
	}

	e := &index.Entry{
		Hash:       h,
		Name:       name,
		Mode:       mode,
		ModifiedAt: fi.ModTime(),
		Size:       uint64(fi.Size()),
	}

	// if the FileInfo.Sys() comes from os the ctime, dev, inode, uid and gid
	// can be retrieved, otherwise this doesn't apply
	if fillSystemInfo != nil {
		fillSystemInfo(e, fi.Sys())
	}
	idx.Add(e)
	return nil
}

var fillSystemInfo func(e *index.Entry, sys any)

// Clean the worktree by removing untracked files.
// An empty dir could be removed - this is what  `zeta clean -f -d .` does.
func (w *Worktree) Clean(ctx context.Context, opts *CleanOptions) error {
	s, err := w.Status(ctx, false)
	if err != nil {
		return err
	}

	root := ""
	files, err := w.fs.ReadDir(root)
	if err != nil {
		return err
	}
	m := ignore.NewMatcher([]ignore.Pattern{})
	return w.doClean(s, m, opts, root, files)
}

func (w *Worktree) doClean(status Status, matcher ignore.Matcher, opts *CleanOptions, dir string, files []fs.DirEntry) error {
	for _, fi := range files {
		if fi.Name() == ZetaDirName {
			continue
		}

		// relative path under the root
		path := filepath.Join(dir, fi.Name())
		if fi.IsDir() {
			if !opts.Dir {
				continue
			}

			subfiles, err := w.fs.ReadDir(path)
			if err != nil {
				return err
			}
			err = w.doClean(status, matcher, opts, path, subfiles)
			if err != nil {
				return err
			}
			continue
		}
		if status.IsUntracked(path) || (opts.All && matcher.Match(strings.Split(path, string(os.PathSeparator)), false)) {
			if opts.DryRun {
				fmt.Fprintf(os.Stderr, "%s %s\n", W("Would remove"), path)
				continue
			}
			fmt.Fprintf(os.Stderr, "%s %s\n", W("Removing"), path)
			if err := w.fs.Remove(path); err != nil {
				return err
			}
		}
	}

	if opts.Dir && dir != "" {
		return w.removeEmptyDirectory(dir)
	}

	return nil
}

// GrepResult is structure of a grep result.
type GrepResult struct {
	// FileName is the name of file which contains match.
	FileName string
	// LineNumber is the line number of a file at which a match was found.
	LineNumber int
	// Content is the content of the file at the matching line.
	Content string
	// TreeName is the name of the tree (reference name/commit hash) at
	// which the match was performed.
	TreeName string
}

func (gr GrepResult) String() string {
	return fmt.Sprintf("%s:%s:%d:%s", gr.TreeName, gr.FileName, gr.LineNumber, gr.Content)
}

// Grep performs grep on a repository.
func (r *Repository) Grep(ctx context.Context, opts *GrepOptions) ([]GrepResult, error) {
	if err := opts.validate(r); err != nil {
		return nil, err
	}

	// Obtain commit hash from options (CommitHash or ReferenceName).
	var commitHash plumbing.Hash
	// treeName contains the value of TreeName in GrepResult.
	var treeName string

	if opts.ReferenceName != "" {
		ref, err := r.ReferenceResolve(opts.ReferenceName)
		if err != nil {
			return nil, err
		}
		commitHash = ref.Hash()
		treeName = opts.ReferenceName.String()
	} else if !opts.CommitHash.IsZero() {
		commitHash = opts.CommitHash
		treeName = opts.CommitHash.String()
	}

	// Obtain a tree from the commit hash and get a tracked files iterator from
	// the tree.
	tree, err := r.getTreeFromCommitHash(ctx, commitHash)
	if err != nil {
		return nil, err
	}
	fileiter := tree.Files()

	return findMatchInFiles(ctx, fileiter, treeName, opts)
}

// Grep performs grep on a worktree.
func (w *Worktree) Grep(ctx context.Context, opts *GrepOptions) ([]GrepResult, error) {
	return w.Repository.Grep(ctx, opts)
}

// findMatchInFiles takes a FileIter, worktree name and GrepOptions, and
// returns a slice of GrepResult containing the result of regex pattern matching
// in content of all the files.
func findMatchInFiles(ctx context.Context, fileiter *object.FileIter, treeName string, opts *GrepOptions) ([]GrepResult, error) {
	var results []GrepResult

	err := fileiter.ForEach(ctx, func(file *object.File) error {
		var fileInPathSpec bool

		// When no pathspecs are provided, search all the files.
		if len(opts.PathSpecs) == 0 {
			fileInPathSpec = true
		}

		// Check if the file name matches with the pathspec. Break out of the
		// loop once a match is found.
		for _, pathSpec := range opts.PathSpecs {
			if pathSpec != nil && pathSpec.MatchString(file.Name) {
				fileInPathSpec = true
				break
			}
		}

		// If the file does not match with any of the pathspec, skip it.
		if !fileInPathSpec {
			return nil
		}

		if file.Size > opts.Limit {
			// Ignore large file
			return nil
		}

		grepResults, err := findMatchInFile(ctx, file, treeName, opts)
		if err != nil {
			return err
		}
		results = append(results, grepResults...)

		return nil
	})

	return results, err
}

// findMatchInFile takes a single File, worktree name and GrepOptions,
// and returns a slice of GrepResult containing the result of regex pattern
// matching in the given file.
func findMatchInFile(ctx context.Context, file *object.File, treeName string, opts *GrepOptions) ([]GrepResult, error) {
	var grepResults []GrepResult

	rc, _, err := file.OriginReader(ctx)
	if err != nil {
		return grepResults, err
	}
	defer rc.Close()

	br := bufio.NewScanner(rc)
	for lineNum := 0; br.Scan(); lineNum++ {
		cnt := br.Text()
		addToResult := false

		// Match the patterns and content. Break out of the loop once a
		// match is found.
		for _, pattern := range opts.Patterns {
			if pattern != nil && pattern.MatchString(cnt) {
				// Add to result only if invert match is not enabled.
				if !opts.InvertMatch {
					addToResult = true
					break
				}
			} else if opts.InvertMatch {
				// If matching fails, and invert match is enabled, add to
				// results.
				addToResult = true
				break
			}
		}

		if addToResult {
			grepResults = append(grepResults, GrepResult{
				FileName:   file.Name,
				LineNumber: lineNum + 1,
				Content:    cnt,
				TreeName:   treeName,
			})
		}
	}
	return grepResults, nil
}

// will walk up the directory tree removing all encountered empty
// directories, not just the one containing this file
func rmFileAndDirsIfEmpty(fs vfs.VFS, name string) error {
	if len(name) == 0 {
		return nil
	}
	if err := fs.RemoveAll(name); err != nil {
		return err
	}
	dir := filepath.Dir(name)
	for {
		removed, err := removeDirIfEmpty(fs, dir)
		if err != nil {
			return err
		}

		if !removed {
			// directory was not empty and not removed,
			// stop checking parents
			break
		}

		// move to parent directory
		dir = filepath.Dir(dir)
	}

	return nil
}

// removeDirIfEmpty will remove the supplied directory `dir` if
// `dir` is empty
// returns true if the directory was removed
func removeDirIfEmpty(fs vfs.VFS, dir string) (bool, error) {
	files, err := fs.ReadDir(dir)
	if err != nil {
		return false, err
	}

	if len(files) > 0 {
		return false, nil
	}

	err = fs.Remove(dir)
	if err != nil {
		return false, err
	}

	return true, nil
}

type indexBuilder struct {
	entries map[string]*index.Entry
}

func newIndexBuilder(idx *index.Index) *indexBuilder {
	entries := make(map[string]*index.Entry, len(idx.Entries))
	for _, e := range idx.Entries {
		entries[e.Name] = e
	}
	return &indexBuilder{
		entries: entries,
	}
}

func newUnlessIndexBuilder(idx *index.Index, m *Matcher) *indexBuilder {
	entries := make(map[string]*index.Entry, len(idx.Entries))
	for _, e := range idx.Entries {
		if m.Match(e.Name) {
			continue
		}
		entries[e.Name] = e
	}
	return &indexBuilder{
		entries: entries,
	}
}

func (b *indexBuilder) Write(idx *index.Index) {
	idx.Entries = idx.Entries[:0]
	for _, e := range b.entries {
		idx.Entries = append(idx.Entries, e)
	}
}

func (b *indexBuilder) Add(e *index.Entry) {
	b.entries[e.Name] = e
}

func (b *indexBuilder) Remove(name string) {
	delete(b.entries, filepath.ToSlash(name))
}
