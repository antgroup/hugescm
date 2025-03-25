// Copyright (c) 2017- GitHub, Inc. and Git LFS contributors
// SPDX-License-Identifier: MIT

package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
)

type Set interface {
	Object(name plumbing.Hash) (*SizeReader, error)
	Exists(name plumbing.Hash) error
	Search(prefix plumbing.Hash) (plumbing.Hash, error)
	Close() error
}

type set struct {
	// m maps the leading byte of a BLAKE3 object name to a set of packfiles
	// that might contain that object, in order of which packfile is most
	// likely to contain that object.
	m map[byte][]*Packfile

	// closeFn is a function that is run by Close(), designated to free
	// resources held by the *Set, like open packfiles.
	closeFn func() error
}

var (
	_ Set = &set{}
)

// Close closes all open packfiles, returning an error if one was encountered.
func (s *set) Close() error {
	if s.closeFn == nil {
		return nil
	}
	return s.closeFn()
}

// iterFn is a function that takes a given packfile and opens an object from it.
type iterFn func(p *Packfile) (r *SizeReader, err error)

func (s *set) Object(name plumbing.Hash) (*SizeReader, error) {
	return s.each(name, func(p *Packfile) (*SizeReader, error) {
		return p.Object(name)
	})
}

func (s *set) each(name plumbing.Hash, fn iterFn) (*SizeReader, error) {
	k := name[0]
	for _, pack := range s.m[k] {
		o, err := fn(pack)
		if err != nil {
			if IsNotFound(err) {
				continue
			}
			return nil, err
		}
		return o, nil
	}

	return nil, plumbing.NoSuchObject(name)
}

func (s *set) Exists(name plumbing.Hash) error {
	return s.eachExists(name, func(p *Packfile) error {
		return p.Exists(name)
	})
}

func (s *set) eachExists(name plumbing.Hash, fn func(*Packfile) error) error {
	k := name[0]
	for _, pack := range s.m[k] {
		err := fn(pack)
		if err != nil {
			if IsNotFound(err) {
				continue
			}
			return err
		}
		return nil
	}
	return plumbing.NoSuchObject(name)
}

type searchFn func(p *Packfile) (oid plumbing.Hash, err error)

func (s *set) Search(prefix plumbing.Hash) (oid plumbing.Hash, err error) {
	return s.eachSearch(prefix, func(p *Packfile) (oid plumbing.Hash, err error) {
		return p.Search(prefix)
	})
}

func (s *set) eachSearch(name plumbing.Hash, fn searchFn) (oid plumbing.Hash, err error) {
	k := name[0]
	for _, pack := range s.m[k] {
		o, err := fn(pack)
		if err != nil {
			if IsNotFound(err) {
				continue
			}
			return oid, err
		}
		return o, nil
	}

	return oid, plumbing.NoSuchObject(name)
}

// packsConcat creates a new *Set from the given packfiles.
func packsConcat(packs ...*Packfile) Set {
	m := make(map[byte][]*Packfile)

	for i := range 256 {
		n := byte(i)

		for j := range packs {
			pack := packs[j]

			var count uint32
			if n == 0 {
				count = pack.idx.fanout[n]
			} else {
				count = pack.idx.fanout[n] - pack.idx.fanout[n-1]
			}

			if count > 0 {
				m[n] = append(m[n], pack)
			}
		}

		sort.Slice(m[n], func(i, j int) bool {
			ni := m[n][i].idx.fanout[n]
			nj := m[n][j].idx.fanout[n]

			return ni > nj
		})
	}

	return &set{
		m: m,
		closeFn: func() error {
			for _, pack := range packs {
				if err := pack.Close(); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

var (
	// nameRe is a regular expression that matches the basename of a
	// filepath that is a packfile.
	//
	// It includes one matchgroup, which is the SHA-1 name of the pack.
	nameRe = regexp.MustCompile(`^(.*)\.pack$`)
)

// globEscapes uses these escapes because filepath.Glob does not understand
// backslash escapes on Windows.
var globEscapes = map[string]string{
	"*": "[*]",
	"?": "[?]",
	"[": "[[]",
}

func escapeGlobPattern(s string) string {
	for char, escape := range globEscapes {
		s = strings.ReplaceAll(s, char, escape)
	}
	return s
}

func newPacks(db string) ([]*Packfile, error) {
	pd := filepath.Join(db, "pack")

	paths, err := filepath.Glob(filepath.Join(escapeGlobPattern(pd), "*.pack"))
	if err != nil {
		return nil, err
	}

	packs := make([]*Packfile, 0, len(paths))

	for _, path := range paths {
		subMatch := nameRe.FindStringSubmatch(filepath.Base(path))
		if len(subMatch) != 2 {
			continue
		}

		name := subMatch[1]

		ifd, err := os.Open(filepath.Join(pd, fmt.Sprintf("%s.idx", name)))
		if err != nil {
			// We have a pack (since it matched the regex), but the
			// index is missing or unusable.  Skip this pack and
			// continue on with the next one, as Git does.
			if ifd != nil {
				// In the unlikely event that we did open a
				// file, close it, but discard any error in
				// doing so.
				ifd.Close()
			}
			continue
		}

		pfd, err := os.Open(filepath.Join(pd, fmt.Sprintf("%s.pack", name)))
		if err != nil {
			_ = ifd.Close()
			return nil, err
		}

		pack, err := DecodePackfile(pfd)
		if err != nil {
			_ = ifd.Close()
			return nil, err
		}

		idx, err := DecodeIndex(ifd)
		if err != nil {
			_ = pack.Close()
			return nil, err
		}

		pack.idx = idx

		packs = append(packs, pack)
	}
	return packs, nil
}

// NewSets
func NewSets(db string) (Set, error) {
	packs, err := newPacks(db)
	if err != nil {
		return nil, err
	}
	return packsConcat(packs...), nil
}

type Packs []*Packfile

func (ps Packs) PackedObjects(recv RecvFunc) error {
	for _, p := range ps {
		if err := p.idx.PackedObjects(recv); err != nil {
			return err
		}
	}
	return nil
}

func NewPacks(db string) (Set, Packs, error) {
	packs, err := newPacks(db)
	if err != nil {
		return nil, nil, err
	}
	return packsConcat(packs...), packs, nil
}
