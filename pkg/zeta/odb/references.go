// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
)

const (
	shallowPath = "shallow"
	// Special references

	MERGE_HEAD       plumbing.ReferenceName = "MERGE_HEAD"
	FETCH_HEAD       plumbing.ReferenceName = "FETCH_HEAD"
	CHERRY_PICK_HEAD plumbing.ReferenceName = "CHERRY_PICK_HEAD"
	AUTO_MERGE       plumbing.ReferenceName = "AUTO_MERGE"
	MERGE_AUTOSTASH  plumbing.ReferenceName = "MERGE_AUTOSTASH"
)

var (
	specialRefs = map[plumbing.ReferenceName]bool{
		MERGE_HEAD:       true,
		FETCH_HEAD:       true,
		CHERRY_PICK_HEAD: true,
	}
	ErrNotSpecialReferenceName = errors.New("not special reference name")
)

func (d *ODB) hashFromFile(name string) (plumbing.Hash, error) {
	p := filepath.Join(d.root, name)
	data, err := os.ReadFile(p)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	line := strings.TrimSpace(string(data))
	return plumbing.NewHash(line), nil
}

func (d *ODB) DeepenFrom() (plumbing.Hash, error) {
	return d.hashFromFile(shallowPath)
}

func (d *ODB) ResolveSpecReference(name plumbing.ReferenceName) (plumbing.Hash, error) {
	return d.hashFromFile(string(name))
}

func (d *ODB) Shallow(oid plumbing.Hash) error {
	shallowPath := filepath.Join(d.root, shallowPath)
	fd, err := os.Create(shallowPath)
	if err != nil {
		return err
	}
	defer fd.Close() // nolint
	if _, err := fd.WriteString(oid.String()); err != nil {
		return err
	}
	return nil
}

func (d *ODB) Unshallow() error {
	shallowPath := filepath.Join(d.root, shallowPath)
	return os.Remove(shallowPath)
}

func openNotExists(name string) (*os.File, error) {
	_ = os.MkdirAll(filepath.Dir(name), 0755)
	return os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_TRUNC, 0644)
}

func (d *ODB) SpecReferenceRemove(name plumbing.ReferenceName) error {
	if !specialRefs[name] {
		return ErrNotSpecialReferenceName
	}
	fileName := filepath.Join(d.root, name.String())
	return os.Remove(fileName)
}

func (d *ODB) SpecReferenceUpdate(name plumbing.ReferenceName, oid plumbing.Hash) error {
	if !specialRefs[name] {
		return ErrNotSpecialReferenceName
	}
	fileName := filepath.Join(d.root, name.String())
	lockName := fileName + ".lock"
	fd, err := openNotExists(lockName)
	if err != nil {
		if os.IsExist(err) {
			return plumbing.NewErrResourceLocked("reference", name)
		}
		return err
	}
	defer func() {
		_ = os.Remove(lockName)
	}()
	if _, err := fd.WriteString(oid.String()); err != nil {
		_ = fd.Close()
		return err
	}
	_ = fd.Close()
	if err := os.Rename(lockName, fileName); err != nil {
		return err
	}
	return nil
}
