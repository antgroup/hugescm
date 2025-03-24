package storage

import (
	"errors"
	"io"

	ge "github.com/antgroup/hugescm/modules/git/gitobj/errors"
)

// Storage implements an interface for reading, but not writing, objects in an
// object database.
type multiStorage struct {
	storages []Storage
}

func MultiStorage(args ...Storage) Storage {
	return &multiStorage{storages: args}
}

// Open returns a handle on an existing object keyed by the given object
// ID.  It returns an error if that file does not already exist.
func (m *multiStorage) Open(oid []byte) (f io.ReadCloser, err error) {
	for _, s := range m.storages {
		f, err := s.Open(oid)
		if err != nil {
			if ge.IsNoSuchObject(err) {
				continue
			}
			return nil, err
		}
		if s.IsCompressed() {
			return newDecompressingReadCloser(f)
		}
		return f, nil
	}
	return nil, ge.NoSuchObject(oid)
}

// Close closes the filesystem, after which no more operations are
// allowed.
func (m *multiStorage) Close() error {
	var errs []error
	for _, s := range m.storages {
		if err := s.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Compressed indicates whether data read from this storage source will
// be zlib-compressed.
func (m *multiStorage) IsCompressed() bool {
	// To ensure we can read from any Storage type, we automatically
	// decompress items if they need it.
	return false
}
