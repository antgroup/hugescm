// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/strengthen"
)

var (
	ErrUnsupportedObject = errors.New("unsupported object type")
)

type ObjectType int8

const (
	InvalidObject ObjectType = 0
	CommitObject  ObjectType = 1
	TreeObject    ObjectType = 2
	BlobObject    ObjectType = 3
	TagObject     ObjectType = 4
	// 5 reserved for future expansion
	OFSDeltaObject  ObjectType = 6
	REFDeltaObject  ObjectType = 7
	FragmentsObject ObjectType = 8 // File Fragmentation Extension

	AnyObject ObjectType = -127
)

func (t ObjectType) String() string {
	switch t {
	case CommitObject:
		return "commit"
	case TreeObject:
		return "tree"
	case BlobObject:
		return "blob"
	case TagObject:
		return "tag"
	case FragmentsObject:
		return "fragments"
	case AnyObject:
		return "any"
	case OFSDeltaObject:
		return "ofs-delta"
	case REFDeltaObject:
		return "ref-delta"
	default:
		return "unknown"
	}
}

// ObjectTypeFromString converts from a given string to an ObjectType
// enumeration instance.
func ObjectTypeFromString(s string) ObjectType {
	switch strings.ToLower(s) {
	case "blob":
		return BlobObject
	case "tree":
		return TreeObject
	case "commit":
		return CommitObject
	case "tag":
		return TagObject
	case "fragments":
		return FragmentsObject
	case "any":
		return AnyObject
	case "ofs-delta":
		return OFSDeltaObject
	case "ref-delta":
		return REFDeltaObject
	default:
		return InvalidObject
	}
}

func (t ObjectType) MarshalJSON() ([]byte, error) {
	return strengthen.BufferCat("\"", t.String(), "\""), nil
}

func (t *ObjectType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = ObjectTypeFromString(s)
	return nil
}

type Reader interface {
	io.Reader
	Hash() plumbing.Hash
	Type() ObjectType
}

type reader struct {
	io.Reader
	hash       plumbing.Hash
	objectType ObjectType
}

func (r *reader) Hash() plumbing.Hash {
	return r.hash
}

func (r *reader) Type() ObjectType {
	return r.objectType
}

const (
	// ZstandardMagic: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#frames
	ZstandardMagic = 0xFD2FB528
)

func isZstandardMagic(magic [4]byte) bool {
	return binary.LittleEndian.Uint32(magic[:]) == ZstandardMagic
}

func Decode(r io.Reader, oid plumbing.Hash, b Backend) (any, error) {
	var magic [4]byte
	n, err := io.ReadFull(r, magic[:])
	if err != nil {
		return nil, err
	}
	if n != 4 {
		return nil, io.EOF
	}
	if isZstandardMagic(magic) {
		zr, err := streamio.GetZstdReader(io.MultiReader(bytes.NewReader(magic[:]), r))
		if err != nil {
			return nil, err
		}
		defer streamio.PutZstdReader(zr)
		r = zr
		if n, err = io.ReadFull(r, magic[:]); err != nil {
			return nil, err
		}
		if n != 4 {
			return nil, io.EOF
		}
	}
	if bytes.Equal(magic[:], COMMIT_MAGIC[:]) {
		c := &Commit{b: b}
		err = c.Decode(&reader{Reader: r, hash: oid, objectType: CommitObject})
		return c, err
	}
	if bytes.Equal(magic[:], TREE_MAGIC[:]) {
		t := &Tree{b: b}
		err = t.Decode(&reader{Reader: r, hash: oid, objectType: TreeObject})
		return t, err
	}
	if bytes.Equal(magic[:], FRAGMENTS_MAGIC[:]) {
		f := &Fragments{b: b}
		err = f.Decode(&reader{Reader: r, hash: oid, objectType: FragmentsObject})
		return f, err
	}
	if bytes.Equal(magic[:], TAG_MAGIC[:]) {
		t := &Tag{}
		err = t.Decode(&reader{Reader: r, hash: oid, objectType: TagObject})
		return t, err
	}
	return nil, ErrUnsupportedObject
}

func Base64Decode(input string, oid plumbing.Hash, b Backend) (any, error) {
	rawBytes, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return nil, err
	}
	return Decode(bytes.NewReader(rawBytes), oid, b)
}

func Base64DecodeAs[T Commit | Tree | Fragments | Tag](input string, oid plumbing.Hash, b Backend) (*T, error) {
	rawBytes, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return nil, err
	}
	a, err := Decode(bytes.NewReader(rawBytes), oid, b)
	if err != nil {
		return nil, err
	}
	if v, ok := a.(*T); ok {
		return v, nil
	}
	return nil, ErrUnsupportedObject
}

func HashObject(r io.Reader) (plumbing.Hash, ObjectType, error) {
	var magic [4]byte
	n, err := io.ReadFull(r, magic[:])
	if err != nil {
		return plumbing.ZeroHash, InvalidObject, err
	}
	if n != 4 {
		return plumbing.ZeroHash, InvalidObject, io.EOF
	}
	if isZstandardMagic(magic) {
		zr, err := streamio.GetZstdReader(io.MultiReader(bytes.NewReader(magic[:]), r))
		if err != nil {
			return plumbing.ZeroHash, InvalidObject, err
		}
		defer streamio.PutZstdReader(zr)
		r = zr
		if n, err = io.ReadFull(r, magic[:]); err != nil {
			return plumbing.ZeroHash, InvalidObject, err
		}
		if n != 4 {
			return plumbing.ZeroHash, InvalidObject, io.EOF
		}
	}
	var t ObjectType
	switch {
	case bytes.Equal(magic[:], TREE_MAGIC[:]):
		t = TreeObject
	case bytes.Equal(magic[:], COMMIT_MAGIC[:]):
		t = CommitObject
	case bytes.Equal(magic[:], FRAGMENTS_MAGIC[:]):
		t = FragmentsObject
	case bytes.Equal(magic[:], TAG_MAGIC[:]):
		t = TagObject
	default:
		return plumbing.ZeroHash, InvalidObject, fmt.Errorf("unsupport magic '%08x'", magic[:])
	}
	hasher := plumbing.NewHasher()
	if _, err := io.Copy(hasher, io.MultiReader(bytes.NewReader(magic[:]), r)); err != nil {
		return plumbing.ZeroHash, InvalidObject, err
	}
	return hasher.Sum(), t, nil
}

type Encoder interface {
	Encode(io.Writer) error
}

func Base64Encode(e Encoder) (string, error) {
	var b bytes.Buffer
	if err := e.Encode(&b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

type Printer interface {
	Pretty(io.Writer) error
}

func Hash(e Encoder) plumbing.Hash {
	h := plumbing.NewHasher()
	if err := e.Encode(h); err != nil {
		return plumbing.ZeroHash
	}
	return h.Sum()
}

func NewSnapshotCommit(cc *Commit, b Backend) *Commit {
	return &Commit{
		Hash:         cc.Hash,
		Author:       cc.Author,
		Committer:    cc.Committer,
		Parents:      cc.Parents,
		Tree:         cc.Tree,
		ExtraHeaders: cc.ExtraHeaders,
		Message:      cc.Message,
		b:            b,
	}
}

func NewSnapshotTree(t *Tree, b Backend) *Tree {
	entries := make([]*TreeEntry, 0, len(t.Entries))
	for _, e := range t.Entries {
		entries = append(entries, e.Clone())
	}
	return &Tree{
		Hash:    t.Hash,
		Entries: entries,
		b:       b,
	}
}
