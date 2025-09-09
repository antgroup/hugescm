// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"io"
	"sort"

	"github.com/antgroup/hugescm/modules/binary"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
)

var (
	FRAGMENTS_MAGIC = [4]byte{'Z', 'F', 0x00, 0x01}
)

type Fragment struct {
	Index uint32        `json:"index"`
	Size  uint64        `json:"size"`
	Hash  plumbing.Hash `json:"hash"`
}

type FragmentsOrder []*Fragment

// Len implements sort.Interface.Len() and return the length of the underlying
// slice.
func (s FragmentsOrder) Len() int { return len(s) }

// Swap implements sort.Interface.Swap() and swaps the two elements at i and j.
func (s FragmentsOrder) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Less implements sort.Interface.Less() and returns whether the element at "i"
// is compared as "less" than the element at "j". In other words, it returns if
// the element at "i" should be sorted ahead of that at "j".
//
// It performs this comparison in lexicographic byte-order according to the
// rules above (see FragmentsOrder).
func (s FragmentsOrder) Less(i, j int) bool {
	return s[i].Index < s[j].Index
}

type Fragments struct {
	Hash    plumbing.Hash // NOT Encode
	Size    uint64
	Origin  plumbing.Hash // origin file hash checksum
	Entries []*Fragment
	b       Backend
}

func (f *Fragments) Encode(w io.Writer) error {
	sort.Sort(FragmentsOrder(f.Entries)) // sort
	_, err := w.Write(FRAGMENTS_MAGIC[:])
	if err != nil {
		return err
	}
	if err := binary.WriteUint64(w, f.Size); err != nil {
		return err
	}
	if _, err = w.Write(f.Origin[:]); err != nil {
		return err
	}
	for _, entry := range f.Entries {
		if err := binary.WriteUint32(w, entry.Index); err != nil {
			return err
		}
		if err := binary.WriteUint64(w, entry.Size); err != nil {
			return err
		}
		if _, err = w.Write(entry.Hash[:]); err != nil {
			return err
		}
	}
	return nil
}

func (f *Fragments) Decode(reader Reader) error {
	if reader.Type() != FragmentsObject {
		return ErrUnsupportedObject
	}
	f.Hash = reader.Hash()
	r := streamio.GetBufioReader(reader)
	defer streamio.PutBufioReader(r)

	f.Entries = nil
	var err error
	if f.Size, err = binary.ReadUint64(r); err != nil {
		return err
	}
	if _, err = io.ReadFull(r, f.Origin[:]); err != nil {
		return err
	}
	for {
		entry := new(Fragment)
		if entry.Index, err = binary.ReadUint32(r); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if entry.Size, err = binary.ReadUint64(r); err != nil {
			return err
		}
		if _, err = io.ReadFull(r, entry.Hash[:]); err != nil {
			return err
		}
		f.Entries = append(f.Entries, entry)
	}
	return nil
}
