// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package refs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
)

const (
	suffix              = ".zeta"
	packedRefsPath      = "packed-refs"
	configPath          = "config"
	indexPath           = "index"
	shallowPath         = "shallow"
	modulePath          = "modules"
	objectsPath         = "objects"
	packPath            = "pack"
	refsPath            = "refs"
	branchesPath        = "branches"
	hooksPath           = "hooks"
	infoPath            = "info"
	remotesPath         = "remotes"
	logsPath            = "logs"
	worktreesPath       = "worktrees"
	tmpPackedRefsPrefix = "._packed-refs"

	// packPrefix = "pack-"
	// packExt    = ".pack"
	// idxExt     = ".idx"
)

var (
	ErrReferenceHasChanged = errors.New("reference has changed concurrently")
)

type fsBackend struct {
	repoPath string
}

func NewBackend(repoPath string) Backend {
	return &fsBackend{repoPath: repoPath}
}

func (b *fsBackend) HEAD() (*plumbing.Reference, error) {
	return b.readRefFromHEAD()
}

func (b *fsBackend) References() (*DB, error) {
	db := &DB{cache: make(map[plumbing.ReferenceName]*plumbing.Reference), references: make([]*plumbing.Reference, 0, 100)}
	var err error
	if err = b.addRefsFromRefDir(db); err != nil {
		return nil, err
	}
	if err := b.addRefsFromPackedRefs(db); err != nil {
		return nil, err
	}
	if db.head, err = b.readRefFromHEAD(); err != nil {
		return nil, err
	}
	return db, nil
}

func (b *fsBackend) addRefsFromRefDir(db *DB) error {
	return b.walkReferencesTree(refsPath, db)
}

func (b *fsBackend) addRefsFromPackedRefs(db *DB) error {
	fd, err := os.Open(filepath.Join(b.repoPath, packedRefsPath))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	defer fd.Close()
	s := bufio.NewScanner(fd)
	for s.Scan() {
		ref, err := b.processLine(s.Text())
		if err != nil {
			return err
		}
		if ref == nil {
			continue
		}
		if _, ok := db.cache[ref.Name()]; !ok {
			db.references = append(db.references, ref)
			db.cache[ref.Name()] = ref
		}
	}
	return s.Err()
}

func (b *fsBackend) readRefFromHEAD() (*plumbing.Reference, error) {
	ref, err := b.readReferenceFile("HEAD")
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func (b *fsBackend) walkReferencesTree(prefix string, db *DB) error {
	files, err := os.ReadDir(filepath.Join(b.repoPath, prefix))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, f := range files {
		newPrefix := prefix + "/" + f.Name() // always use unix '/'
		if f.IsDir() {
			if err = b.walkReferencesTree(newPrefix, db); err != nil {
				return err
			}
			continue
		}
		ref, err := b.readReferenceFile(newPrefix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if ref != nil {
			if _, ok := db.cache[ref.Name()]; !ok {
				db.references = append(db.references, ref)
				db.cache[ref.Name()] = ref
			}
		}
	}
	return nil
}

func (b *fsBackend) readReferenceFile(refname string) (ref *plumbing.Reference, err error) {
	p := filepath.Join(b.repoPath, refname)
	si, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if si.IsDir() {
		return nil, ErrIsDir
	}
	fd, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	return b.readReferenceFrom(fd, refname)
}

func (b *fsBackend) readReferenceMatchPrefix(prefix string) (ref *plumbing.Reference, err error) {
	refPath := filepath.Join(b.repoPath, prefix)
	si, err := os.Stat(refPath)
	if err != nil {
		return nil, err
	}
	if !si.IsDir() {
		fd, err := os.Open(refPath)
		if err != nil {
			return nil, err
		}
		defer fd.Close()
		return b.readReferenceFrom(fd, prefix)
	}
	var refname string
	err = filepath.WalkDir(refPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		refname, err = filepath.Rel(b.repoPath, path)
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(refname) == 0 {
		return nil, nil
	}
	fd, err := os.Open(filepath.Join(b.repoPath, refname))
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	return b.readReferenceFrom(fd, refname)
}

func (b *fsBackend) readReferenceFrom(rd io.Reader, name string) (ref *plumbing.Reference, err error) {
	data, err := io.ReadAll(rd)
	if err != nil {
		return nil, err
	}
	line := strings.TrimSpace(string(data))
	return plumbing.NewReferenceFromStrings(name, line), nil
}

func (b *fsBackend) processLine(line string) (*plumbing.Reference, error) {
	if len(line) == 0 {
		return nil, nil
	}

	switch line[0] {
	case '#': // comment - ignore
		return nil, nil
	case '^': // annotated tag commit of the previous line - ignore
		return nil, nil
	default:
		target, name, ok := strings.Cut(line, " ") // hash then ref
		if !ok {
			return nil, ErrPackedRefsBadFormat
		}

		return plumbing.NewReferenceFromStrings(name, target), nil
	}
}

func (b *fsBackend) matchReferenceName(line string, want string) (*plumbing.Reference, error) {
	if len(line) == 0 {
		return nil, nil
	}

	switch line[0] {
	case '#': // comment - ignore
		return nil, nil
	case '^': // annotated tag commit of the previous line - ignore
		return nil, nil
	default:
		target, name, ok := strings.Cut(line, " ") // hash then ref
		if !ok {
			return nil, ErrPackedRefsBadFormat
		}
		if want != name {
			return nil, nil
		}
		return plumbing.NewReferenceFromStrings(name, target), nil
	}
}

func (b *fsBackend) packedRef(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	fd, err := os.Open(filepath.Join(b.repoPath, packedRefsPath))
	if os.IsNotExist(err) {
		return nil, plumbing.ErrReferenceNotFound
	}
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	s := bufio.NewScanner(fd)
	for s.Scan() {
		ref, err := b.matchReferenceName(s.Text(), string(name))
		if err != nil {
			return nil, err
		}
		if ref != nil {
			return ref, nil
		}
	}

	return nil, plumbing.ErrReferenceNotFound
}

func prefixMatch(name, prefix string) bool {
	prefixLen := len(prefix)
	return len(name) >= prefixLen && name[0:prefixLen] == prefix && (len(name) == prefixLen || name[prefixLen] == '/')
}

func (b *fsBackend) matchReferenceNamePrefix(line string, prefix string) (*plumbing.Reference, error) {
	if len(line) == 0 {
		return nil, nil
	}

	switch line[0] {
	case '#': // comment - ignore
		return nil, nil
	case '^': // annotated tag commit of the previous line - ignore
		return nil, nil
	default:
		target, name, ok := strings.Cut(line, " ") // hash then ref
		if !ok {
			return nil, ErrPackedRefsBadFormat
		}
		if !prefixMatch(name, prefix) {
			return nil, nil
		}
		return plumbing.NewReferenceFromStrings(name, target), nil
	}
}

func (b *fsBackend) matchPackedRefPrefix(prefix plumbing.ReferenceName) (*plumbing.Reference, error) {
	fd, err := os.Open(filepath.Join(b.repoPath, packedRefsPath))
	if os.IsNotExist(err) {
		return nil, plumbing.ErrReferenceNotFound
	}
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	s := bufio.NewScanner(fd)
	for s.Scan() {
		ref, err := b.matchReferenceNamePrefix(s.Text(), string(prefix))
		if err != nil {
			return nil, err
		}
		if ref != nil {
			return ref, nil
		}
	}

	return nil, plumbing.ErrReferenceNotFound
}

func (b *fsBackend) Reference(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	ref, err := b.readReferenceFile(string(name))
	if err == nil {
		return ref, nil
	}
	return b.packedRef(name)
}

func (b *fsBackend) ReferencePrefixMatch(prefix plumbing.ReferenceName) (*plumbing.Reference, error) {
	ref, err := b.readReferenceMatchPrefix(string(prefix))
	if err == nil {
		return ref, nil
	}
	return b.matchPackedRefPrefix(prefix)
}

func (b *fsBackend) checkReference(old *plumbing.Reference) error {
	if old == nil {
		return nil
	}
	ref, err := b.Reference(old.Name())
	if err != nil {
		return err
	}
	if ref.Hash() != old.Hash() {
		return ErrReferenceHasChanged
	}
	return nil
}

func openNotExists(name string) (*os.File, error) {
	_ = os.MkdirAll(filepath.Dir(name), 0755)
	return os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_TRUNC, 0644)
}

func (b *fsBackend) lockPackedRefs(fn func() error) error {
	lockName := filepath.Join(b.repoPath, packedRefsPath+".lock")
	fd, err := openNotExists(lockName)
	if err != nil {
		if os.IsExist(err) {
			return plumbing.NewErrResourceLocked("reference", "packed-refs")
		}
		return err
	}
	err = fn()
	_ = fd.Close()
	_ = os.Remove(lockName)
	return err
}

func CheckClose(c io.Closer, err *error) {
	if closeErr := c.Close(); closeErr != nil && *err == nil {
		*err = closeErr
	}
}

func (b *fsBackend) rewritePackedRefsWithoutRef(name plumbing.ReferenceName) error {
	var tmpName string
	defer func() {
		if len(tmpName) != 0 {
			_ = os.Remove(tmpName)
		}
	}()
	packedRefs := filepath.Join(b.repoPath, packedRefsPath)
	rewriteNeed, err := func() (bool, error) {
		fd, err := os.Open(packedRefs)
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		defer fd.Close()
		tmp, err := os.CreateTemp(b.repoPath, tmpPackedRefsPrefix)
		if err != nil {
			return false, err
		}
		defer tmp.Close()
		_ = tmp.Chmod(0644)
		tmpName = tmp.Name()
		s := bufio.NewScanner(fd)
		found := false
		for s.Scan() {
			line := s.Text()
			ref, err := b.processLine(line)
			if err != nil {
				return false, err
			}
			if ref != nil && ref.Name() == name {
				found = true
				continue
			}
			if _, err := fmt.Fprintln(tmp, line); err != nil {
				return false, err
			}
		}
		if err := s.Err(); err != nil {
			return false, err
		}
		return found, nil
	}()
	if err != nil {
		return err
	}
	if !rewriteNeed {
		return nil
	}
	return os.Rename(tmpName, packedRefs)
}

func (b *fsBackend) ReferenceRemove(r *plumbing.Reference) error {
	fileName := filepath.Join(b.repoPath, r.Name().String())
	lockName := fileName + ".lock"
	fd, err := openNotExists(lockName)
	if err != nil {
		if os.IsExist(err) {
			return plumbing.NewErrResourceLocked("reference", r.Name())
		}
		return err
	}
	_ = fd.Close()
	defer func() {
		_ = os.Remove(lockName)
		_ = b.prune()
	}()
	if err = os.Remove(fileName); err != nil && !os.IsNotExist(err) {
		return err
	}
	return b.lockPackedRefs(func() error {
		return b.rewritePackedRefsWithoutRef(r.Name())
	})
}

func (b *fsBackend) ReferenceUpdate(r, old *plumbing.Reference) error {
	var content string
	switch r.Type() {
	case plumbing.SymbolicReference:
		content = fmt.Sprintf("ref: %s\n", r.Target())
	case plumbing.HashReference:
		content = fmt.Sprintln(r.Hash().String())
	}
	fileName := filepath.Join(b.repoPath, r.Name().String())
	lockName := fileName + ".lock"
	fd, err := openNotExists(lockName)
	if err != nil {
		if os.IsExist(err) {
			return plumbing.NewErrResourceLocked("reference", r.Name())
		}
		return err
	}
	defer func() {
		_ = os.Remove(lockName)
	}()
	if err := b.checkReference(old); err != nil {
		_ = fd.Close()
		return err
	}
	if _, err := fd.WriteString(content); err != nil {
		_ = fd.Close()
		return err
	}
	_ = fd.Close()
	if err := os.Rename(lockName, fileName); err != nil {
		return err
	}
	return nil
}

func (b *fsBackend) rewritePackedRefs() error {
	// Gather all refs using addRefsFromRefDir and addRefsFromPackedRefs.
	db := &DB{cache: make(map[plumbing.ReferenceName]*plumbing.Reference), references: make([]*plumbing.Reference, 0, 100)}
	if err := b.addRefsFromRefDir(db); err != nil {
		return err
	}
	if len(db.references) == 0 {
		// Nothing to do!
		return nil
	}
	looseRefs := slices.Clone(db.references)
	if err := b.addRefsFromPackedRefs(db); err != nil {
		return err
	}
	var tempPackedRefs string
	defer func() {
		if len(tempPackedRefs) != 0 {
			_ = os.Remove(tempPackedRefs)
		}
	}()
	db.Sort()
	err := func() error {
		tmp, err := os.CreateTemp(b.repoPath, tmpPackedRefsPrefix)
		if err != nil {
			return err
		}
		defer tmp.Close()

		tempPackedRefs = tmp.Name()
		w := bufio.NewWriter(tmp)
		_, err = w.WriteString("# pack-refs with: sorted\n")
		if err != nil {
			return err
		}
		for _, ref := range db.references {
			_, err = w.WriteString(ref.String() + "\n")
			if err != nil {
				return err
			}
		}
		err = w.Flush()
		if err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return err
	}
	packedRefs := filepath.Join(b.repoPath, packedRefsPath)
	if err := os.Rename(tempPackedRefs, packedRefs); err != nil {
		return err
	}
	for _, ref := range looseRefs {
		refPath := filepath.Join(b.repoPath, ref.Name().String())
		err = os.Remove(refPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (b *fsBackend) Packed() error {
	if err := b.lockPackedRefs(b.rewritePackedRefs); err != nil {
		return err
	}
	_ = b.prune()
	return nil
}

var (
	pruneKeeps = map[string]bool{
		"heads":   true,
		"tags":    true,
		"remotes": true,
	}
)

func (b *fsBackend) prune() error {
	refsPath := filepath.Join(b.repoPath, "refs")
	entries, err := os.ReadDir(refsPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		absPath := filepath.Join(refsPath, e.Name())
		if err := pruneDirsDFS(absPath, pruneKeeps[e.Name()]); err != nil {
			return err
		}
	}
	return nil
}

func pruneDirsDFS(dir string, keep bool) error {
	empty := true
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			empty = false
			continue
		}
		absPath := filepath.Join(dir, e.Name())
		if err := pruneDirsDFS(absPath, false); err != nil {
			return err
		}
	}
	if !empty || keep {
		return nil
	}
	return os.Remove(dir)
}
