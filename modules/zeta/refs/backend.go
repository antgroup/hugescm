// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package refs

import (
	"errors"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
)

type Backend interface {
	// Find the current reference
	HEAD() (*plumbing.Reference, error)
	// view all references
	References() (*DB, error)
	// Look up a reference using the full reference name.
	Reference(name plumbing.ReferenceName) (*plumbing.Reference, error)
	// ReferencePrefixMatch match reference prefix
	//   prefix: refs/logs
	//   refs/logs ✅
	//   refs/logs/211 ✅
	//   refs/logs.l ❌
	ReferencePrefixMatch(prefix plumbing.ReferenceName) (*plumbing.Reference, error)
	// Update reference
	ReferenceUpdate(r, old *plumbing.Reference) error
	// remove reference
	ReferenceRemove(r *plumbing.Reference) error
	// packed references
	Packed() error
}

func ReferencesDB(repoPath string) (*DB, error) {
	return NewBackend(repoPath).References()
}

const MaxResolveRecursion = 1024

// ErrMaxResolveRecursion is returned by ResolveReference is MaxResolveRecursion
// is exceeded
var ErrMaxResolveRecursion = errors.New("max. recursion level reached")

func ReferenceResolve(b Backend, name plumbing.ReferenceName) (ref *plumbing.Reference, err error) {
	for range MaxResolveRecursion {
		if ref, err = b.Reference(name); err != nil {
			return nil, err
		}
		if ref.Type() != plumbing.SymbolicReference {
			return ref, nil
		}
		name = ref.Target()
	}
	return nil, ErrMaxResolveRecursion
}

// ReferenceIter is a generic closable interface for iterating over references.
type ReferenceIter interface {
	Next() (*plumbing.Reference, error)
	ForEach(func(*plumbing.Reference) error) error
	Close()
}

type ReferenceSliceIter struct {
	series []*plumbing.Reference
	pos    int
}

// NewReferenceSliceIter returns a reference iterator for the given slice of
// objects.
func NewReferenceSliceIter(series []*plumbing.Reference) ReferenceIter {
	return &ReferenceSliceIter{
		series: series,
	}
}

// Next returns the next reference from the iterator. If the iterator has
// reached the end it will return io.EOF as an error.
func (iter *ReferenceSliceIter) Next() (*plumbing.Reference, error) {
	if iter.pos >= len(iter.series) {
		return nil, io.EOF
	}

	obj := iter.series[iter.pos]
	iter.pos++
	return obj, nil
}

// ForEach call the cb function for each reference contained on this iter until
// an error happens or the end of the iter is reached. If ErrStop is sent
// the iteration is stop but no error is returned. The iterator is closed.
func (iter *ReferenceSliceIter) ForEach(cb func(*plumbing.Reference) error) error {
	return forEachReferenceIter(iter, cb)
}

type bareReferenceIterator interface {
	Next() (*plumbing.Reference, error)
	Close()
}

func forEachReferenceIter(iter bareReferenceIterator, cb func(*plumbing.Reference) error) error {
	defer iter.Close()
	for {
		obj, err := iter.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		if err := cb(obj); err != nil {
			if err == plumbing.ErrStop {
				return nil
			}

			return err
		}
	}
}

// Close releases any resources used by the iterator.
func (iter *ReferenceSliceIter) Close() {
	iter.pos = len(iter.series)
}

func NewReferenceIter(b Backend) (ReferenceIter, error) {
	d, err := b.References()
	if err != nil {
		return nil, err
	}
	return NewReferenceSliceIter(d.References()), nil
}
