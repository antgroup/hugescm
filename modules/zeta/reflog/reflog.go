// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package reflog

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

const (
	REFLOG_DIR       = "logs"
	REFLOG_DIR_MOD   = 0777
	REFLOG_FILE_MODE = 0666
)

type Entry struct {
	O, N      plumbing.Hash
	Committer object.Signature
	Message   string
}

type Entries []*Entry

type Reflog struct {
	name    plumbing.ReferenceName
	Entries Entries
}

func (o *Reflog) Empty() bool {
	return o == nil || len(o.Entries) == 0
}

func (o *Reflog) Clear() {
	o.Entries = o.Entries[:0]
}

func (o *Reflog) Drop(index int, rewritePreviousEntry bool) error {
	count := len(o.Entries)
	if index < 0 || index >= count {
		return fmt.Errorf("no reflog entry at index %d", index)
	}
	newEntries := make([]*Entry, 0, count-1)
	for i, e := range o.Entries {
		if i != index {
			newEntries = append(newEntries, e)
		}
	}
	switch {
	case !rewritePreviousEntry || index == 0 || count == 1:
	case index == count-1:
		newEntries[len(newEntries)-1].O = plumbing.ZeroHash
	default:
		newEntries[index-1].O = newEntries[index].N
	}
	o.Entries = newEntries
	return nil
}

// Push New Entry
func (o *Reflog) Push(oid plumbing.Hash, committer *object.Signature, message string) {
	e := &Entry{
		N:         oid,
		Committer: *committer,
		Message:   message,
	}
	newEntries := make([]*Entry, 0, len(o.Entries)+1)
	if len(o.Entries) > 0 {
		e.O = o.Entries[0].N
	}
	newEntries = append(newEntries, e)
	newEntries = append(newEntries, o.Entries...)
	o.Entries = newEntries
}

type DB struct {
	root string
}

func NewDB(root string) *DB {
	return &DB{root: root}
}

var (
	ErrUnparsableReflogLine = errors.New("unparsable reflog line")
)

func newEntry(line string) (*Entry, error) {
	pos := strings.IndexByte(line, ' ')
	if pos == -1 {
		return nil, ErrUnparsableReflogLine
	}
	o := line[0:pos]
	line = line[pos+1:]
	if pos = strings.IndexByte(line, ' '); pos == -1 {
		return nil, ErrUnparsableReflogLine
	}
	n := line[0:pos]
	line = line[pos+1:]
	var message string
	signature := line
	if pos = strings.IndexByte(line, '\t'); pos != -1 {
		message = line[pos+1:]
		signature = line[:pos]
	}
	e := &Entry{
		O:       plumbing.NewHash(o),
		N:       plumbing.NewHash(n),
		Message: message,
	}
	e.Committer.Decode([]byte(signature))
	return e, nil
}

func (d *DB) parse(r io.Reader) ([]*Entry, error) {
	br := bufio.NewScanner(r)
	entries := make([]*Entry, 0, 20)
	for br.Scan() {
		line := strings.TrimSpace(br.Text())
		e, err := newEntry(line)
		if err != nil {
			continue
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return true
	})
	return entries, nil
}

func (d *DB) serialize(w io.Writer, entries []*Entry) error {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if len(e.Message) == 0 {
			if _, err := fmt.Fprintf(w, "%s %s %s\n", e.O, e.N, &e.Committer); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "%s %s %s\t%s\n", e.O, e.N, &e.Committer, strings.ReplaceAll(e.Message, "\n", " ")); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) Exists(refname plumbing.ReferenceName) bool {
	logPath := filepath.Join(d.root, REFLOG_DIR, string(refname))
	if _, err := os.Stat(logPath); err == nil {
		return true
	}
	return false
}

func (d *DB) Read(refname plumbing.ReferenceName) (*Reflog, error) {
	if !plumbing.ValidateReferenceName([]byte(refname)) {
		return nil, plumbing.ErrBadReferenceName{Name: refname.String()}
	}
	logPath := filepath.Join(d.root, REFLOG_DIR, string(refname))
	fd, err := os.Open(logPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(logPath), REFLOG_DIR_MOD); err != nil {
			return nil, err
		}
		if fd, err = os.OpenFile(logPath, os.O_CREATE, REFLOG_FILE_MODE); err != nil {
			return nil, err
		}
		_ = fd.Close()
		return &Reflog{name: refname, Entries: make([]*Entry, 0)}, nil
	}
	defer fd.Close() // nolint
	reflog := &Reflog{
		name: refname,
	}
	if reflog.Entries, err = d.parse(fd); err != nil {
		return nil, err
	}
	return reflog, nil
}

func (d *DB) Write(o *Reflog) error {
	logPath := filepath.Join(d.root, REFLOG_DIR, string(o.name))
	return d.lockPath(o.name, logPath, func() error {
		var tempReflog string
		defer func() {
			if len(tempReflog) != 0 {
				_ = os.Remove(tempReflog)
			}
		}()
		fd, err := os.CreateTemp(filepath.Dir(logPath), "temp_reflog")
		if err != nil {
			return err
		}
		_ = fd.Chmod(0644)
		tempReflog = fd.Name()
		w := bufio.NewWriter(fd)
		if err := d.serialize(w, o.Entries); err != nil {
			_ = fd.Close()
			return err
		}
		if err := w.Flush(); err != nil {
			_ = fd.Close()
			return err
		}
		_ = fd.Close()
		if err := os.Rename(tempReflog, logPath); err != nil {
			return err
		}
		return nil
	})
}

func (d *DB) Rename(oldName, newName plumbing.ReferenceName) error {
	if !plumbing.ValidateReferenceName([]byte(oldName)) {
		return plumbing.ErrBadReferenceName{Name: string(oldName)}
	}
	if !plumbing.ValidateReferenceName([]byte(newName)) {
		return plumbing.ErrBadReferenceName{Name: string(newName)}
	}
	logPathA := filepath.Join(d.root, REFLOG_DIR, string(oldName))
	logPathB := filepath.Join(d.root, REFLOG_DIR, string(newName))
	err := d.lockTowPath(oldName, newName, logPathA, logPathB, func() error {
		return os.Rename(logPathA, logPathB)
	})
	if err == nil || !os.IsExist(err) {
		return err
	}
	logTempPath := filepath.Join(d.root, REFLOG_DIR, "temp_reflog")
	tempName := plumbing.ReferenceName("temp_reflog")
	if err = d.lockTowPath(oldName, tempName, logPathA, logTempPath, func() error {
		return os.Rename(logPathA, logTempPath)
	}); err != nil {
		return err
	}
	_ = d.prune()
	return d.lockTowPath(tempName, newName, logTempPath, logPathB, func() error {
		return os.Rename(logTempPath, logPathA)
	})
}

func (d *DB) Delete(name plumbing.ReferenceName) error {
	if !plumbing.ValidateReferenceName([]byte(name)) {
		return plumbing.ErrBadReferenceName{Name: string(name)}
	}
	logPath := filepath.Join(d.root, REFLOG_DIR, string(name))
	err := d.lockPath(name, logPath, func() error {
		if err := os.Remove(logPath); err != nil && os.IsNotExist(err) {
			return err
		}
		return nil
	})
	_ = d.prune()
	return err
}

func (d *DB) lockPath(refname plumbing.ReferenceName, p string, fn func() error) error {
	lockName := p + ".lock"
	fd, err := openNotExists(lockName)
	if err != nil {
		if os.IsExist(err) {
			return plumbing.NewErrResourceLocked("reflog", refname)
		}
		return err
	}
	err = fn()
	_ = fd.Close()
	_ = os.Remove(lockName)
	return err
}

func (d *DB) lockTowPath(refnameA, refnameB plumbing.ReferenceName, a, b string, fn func() error) error {
	lockNameA := a + ".lock"
	lockNameB := b + ".lock"
	fd1, err := openNotExists(lockNameA)
	if err != nil {
		if os.IsExist(err) {
			return plumbing.NewErrResourceLocked("reflog", refnameA)
		}
		return err
	}
	fd2, err := openNotExists(lockNameB)
	if err != nil {
		_ = fd1.Close()
		_ = os.Remove(lockNameA)
		if os.IsExist(err) {
			return plumbing.NewErrResourceLocked("reflog", refnameB)
		}
		return err
	}
	err = fn()
	_ = fd1.Close()
	_ = os.Remove(lockNameA)
	_ = fd2.Close()
	_ = os.Remove(lockNameB)
	return err
}

func openNotExists(name string) (*os.File, error) {
	_ = os.MkdirAll(filepath.Dir(name), 0755)
	return os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_TRUNC, 0644)
}

var (
	pruneKeeps = map[string]bool{
		"heads":   true,
		"tags":    true,
		"remotes": true,
	}
)

func (d *DB) prune() error {
	logsPath := filepath.Join(d.root, REFLOG_DIR)
	entries, err := os.ReadDir(logsPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		absPath := filepath.Join(logsPath, e.Name())
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
