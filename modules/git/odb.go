package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/antgroup/hugescm/modules/git/gitobj"
)

type ODB struct {
	*gitobj.Database
	tmpdir string
}

func (o *ODB) Close() error {
	err := o.Database.Close()
	_ = os.RemoveAll(o.tmpdir)
	return err
}

// NewODB open repo default odb
func NewODB(repoPath string, hashAlgo HashFormat) (*ODB, error) {
	var options []gitobj.Option
	if hashAlgo != HashUNKNOWN {
		options = append(options, gitobj.ObjectFormat(gitobj.ObjectFormatAlgorithm(hashAlgo.String())))
	}
	objdir := filepath.Join(repoPath, "objects")
	tmpdir, err := NewSundriesDir(repoPath, "objects")
	if err != nil {
		return nil, err
	}
	odb, err := gitobj.NewDatabase(objdir, tmpdir, options...)
	if err != nil {
		_ = os.RemoveAll(tmpdir)
		return nil, err
	}
	if odb.Hasher() == nil {
		_ = os.RemoveAll(tmpdir)
		return nil, fmt.Errorf("unsupported repository hash algorithm %s", hashAlgo)
	}
	return &ODB{Database: odb, tmpdir: tmpdir}, nil
}

var (
	ErrObjectNotFound = errors.New("object not found")
	// ErrInvalidType is returned when an invalid object type is provided.
	ErrInvalidType = errors.New("invalid object type")
)

// ObjectType internal object type
// Integer values from 0 to 7 map to those exposed by git.
// AnyObject is used to represent any from 0 to 7.
type ObjectType int8

const (
	InvalidObject ObjectType = 0
	CommitObject  ObjectType = 1
	TreeObject    ObjectType = 2
	BlobObject    ObjectType = 3
	TagObject     ObjectType = 4
	// 5 reserved for future expansion
	OFSDeltaObject ObjectType = 6
	REFDeltaObject ObjectType = 7

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
	case OFSDeltaObject:
		return "ofs-delta"
	case REFDeltaObject:
		return "ref-delta"
	case AnyObject:
		return "any"
	default:
		return "unknown"
	}
}

func (t ObjectType) Bytes() []byte {
	return []byte(t.String())
}

// Valid returns true if t is a valid ObjectType.
func (t ObjectType) Valid() bool {
	return t >= CommitObject && t <= REFDeltaObject
}

// IsDelta returns true for any ObjectTyoe that represents a delta (i.e.
// REFDeltaObject or OFSDeltaObject).
func (t ObjectType) IsDelta() bool {
	return t == REFDeltaObject || t == OFSDeltaObject
}

// ParseObjectType parses a string representation of ObjectType. It returns an
// error on parse failure.
func ParseObjectType(value string) (typ ObjectType, err error) {
	switch value {
	case "commit":
		typ = CommitObject
	case "tree":
		typ = TreeObject
	case "blob":
		typ = BlobObject
	case "tag":
		typ = TagObject
	case "ofs-delta":
		typ = OFSDeltaObject
	case "ref-delta":
		typ = REFDeltaObject
	default:
		err = ErrInvalidType
	}
	return
}
