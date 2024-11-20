// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/antgroup/hugescm/modules/zeta/backend/pack"
	"github.com/antgroup/hugescm/modules/zeta/backend/storage"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/dgraph-io/ristretto/v2"
)

const (
	DefaultHashALGO        = "BLAKE3"
	DefaultCompressionALGO = "zstd"
)

type Database struct {
	root            string
	sharingRoot     string
	compressionALGO string
	// ro is the locations from which we can read objects.
	metaRO  storage.Storage
	metaRW  storage.WritableStorage
	ro      storage.Storage
	rw      storage.WritableStorage
	metaLRU *ristretto.Cache[string, any]
	// closed is a uint32 managed by sync/atomic's <X>Uint32 methods. It
	// yields a value of 0 if the *Database it is stored upon is open,
	// and a value of 1 if it is closed.
	closed    uint32
	mu        sync.RWMutex
	backend   object.Backend
	enableLRU bool
}

type Option func(*Database)

func WithSharingRoot(sharingRoot string) Option {
	return func(d *Database) {
		if len(sharingRoot) != 0 {
			d.sharingRoot = sharingRoot
		}
	}
}

func WithEnableLRU(enableLRU bool) Option {
	return func(d *Database) {
		d.enableLRU = enableLRU
	}
}

func WithAbstractBackend(backend object.Backend) Option {
	return func(d *Database) {
		d.backend = backend
	}
}

func WithCompressionALGO(compressionALGO string) Option {
	return func(d *Database) {
		if len(compressionALGO) != 0 {
			d.compressionALGO = compressionALGO
		}
	}
}

func (d *Database) Reload() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.initializeMetadataStorage(); err != nil {
		return fmt.Errorf("reload metadata storage error: %w", err)
	}
	if err := d.initializeBlobStorage(); err != nil {
		_ = d.metaRO.Close()
		_ = d.metaRW.Close()
		return fmt.Errorf("reload objects storage error: %w", err)
	}
	return nil
}

func NewDatabase(root string, opts ...Option) (*Database, error) {
	d := &Database{
		root:            root,
		compressionALGO: DefaultCompressionALGO,
	}
	for _, o := range opts {
		o(d)
	}
	if err := d.Reload(); err != nil {
		return nil, err
	}
	if d.backend == nil {
		d.backend = d
	}
	return d, nil
}

func (d *Database) initializeBlobStorage() error {
	if d.ro != nil {
		_ = d.ro.Close()
		d.ro = nil
	}
	if d.rw != nil {
		_ = d.rw.Close()
		d.rw = nil
	}
	zetaDir := d.root
	if len(d.sharingRoot) != 0 {
		zetaDir = d.sharingRoot
	}
	root := filepath.Join(zetaDir, "blob")
	incoming := filepath.Join(zetaDir, "incoming")
	if err := mkdir(root, incoming); err != nil {
		return err
	}
	fsobj := newFileStorer(root, incoming, d.compressionALGO)
	packs, err := pack.NewStorage(root)
	if err != nil {
		return err
	}
	d.ro = storage.MultiStorage(fsobj, packs)
	d.rw = fsobj
	return nil
}

func (d *Database) initializeMetadataStorage() error {
	if d.metaRO != nil {
		_ = d.metaRO.Close()
		d.metaRO = nil
	}
	if d.metaRW != nil {
		_ = d.metaRW.Close()
		d.metaRW = nil
	}
	root := filepath.Join(d.root, "metadata")
	incoming := filepath.Join(d.root, "incoming")
	if err := mkdir(root, incoming); err != nil {
		return err
	}
	fsobj := newFileStorer(root, incoming, d.compressionALGO)
	packs, err := pack.NewStorage(root)
	if err != nil {
		return err
	}
	d.metaRO = storage.MultiStorage(fsobj, packs)
	d.metaRW = fsobj
	if !d.enableLRU {
		return nil
	}
	if d.metaLRU != nil {
		d.metaLRU.Close()
		d.metaLRU = nil
	}
	if d.metaLRU, err = ristretto.NewCache(&ristretto.Config[string, any]{
		NumCounters: 100000,
		MaxCost:     100000,
		BufferItems: 64,
	}); err != nil {
		return err
	}
	return nil
}

func closeSafe(a ...io.Closer) error {
	errs := make([]error, 0, len(a))
	for _, c := range a {
		if c == nil {
			continue
		}
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Close closes the *Database
//
// If Close() has already been called, this function will return an error.
func (d *Database) Close() error {
	if !atomic.CompareAndSwapUint32(&d.closed, 0, 1) {
		return fmt.Errorf("zeta: *Database already closed")
	}
	return closeSafe(d.ro, d.metaRO, d.rw, d.metaRW)
}

func (d *Database) CompressionALGO() string {
	return d.compressionALGO
}

func (d *Database) Root() string {
	return d.root
}
