// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"math"
	"os"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

func (r *Repository) fetchObjects(ctx context.Context, t transport.Transport, target plumbing.Hash, sizeLimit int64, ignoreLarges bool) error {
	if sizeLimit < 0 {
		sizeLimit = math.MaxInt64
	}
	largeSize := r.largeSize()
	larges := make([]*odb.Entry, 0, 100)
	seen := make(map[plumbing.Hash]bool)
	if err := r.odb.CountingSliceObjects(ctx, target, r.Core.SparseDirs, r.maxEntries(), func(ctx context.Context, entries odb.Entries) error {
		smalls := make([]plumbing.Hash, 0, len(entries))
		for _, e := range entries {
			if e.Hash == backend.BLANK_BLOB_HASH {
				continue
			}
			if e.Size > sizeLimit {
				continue
			}
			if e.Size > largeSize {
				if seen[e.Hash] {
					continue
				}
				larges = append(larges, e)
				seen[e.Hash] = true
				continue
			}
			smalls = append(smalls, e.Hash)
		}
		if err := r.batch(ctx, t, smalls); err != nil {
			return err
		}
		if err := r.odb.Reload(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if ignoreLarges {
		return nil
	}
	return r.transfer(ctx, t, larges)
}

func (r *Repository) FetchObjects(ctx context.Context, commit plumbing.Hash, skipLarges bool) error {
	t, err := r.newTransport(ctx, transport.DOWNLOAD)
	if err != nil {
		return err
	}
	if err := r.fetchObjects(ctx, t, commit, NoSizeLimit, skipLarges); err == nil {
		return nil
	}
	if !plumbing.IsNoSuchObject(err) {
		return err
	}
	// Fetch missing commit and objects
	deepenFrom, err := r.odb.DeepenFrom()
	if err != nil && !os.IsNotExist(err) {
		die("resolve shallow: %v", err)
		return err
	}
	return r.fetch(ctx, t, &FetchOptions{Target: commit, DeepenFrom: deepenFrom, SizeLimit: NoSizeLimit})
}

type missingFetcher struct {
	larges  []*odb.Entry
	objects []plumbing.Hash
	seen    map[plumbing.Hash]bool
}

func newMissingFetcher() *missingFetcher {
	return &missingFetcher{
		larges:  make([]*odb.Entry, 0, 100),
		objects: make([]plumbing.Hash, 0, 100),
		seen:    make(map[plumbing.Hash]bool),
	}
}

func (m *missingFetcher) store(o *odb.ODB, oid plumbing.Hash, size int64, largeSize int64) {
	if m.seen[oid] {
		return
	}
	m.seen[oid] = true
	if o.Exists(oid, false) {
		return
	}
	if size > largeSize {
		m.larges = append(m.larges, &odb.Entry{Hash: oid, Size: size})
	} else {
		m.objects = append(m.objects, oid)
	}
}

func (r *Repository) fetchMissingObjects(ctx context.Context, m *missingFetcher, ignoreLarges bool) error {
	if len(m.larges) == 0 && len(m.objects) == 0 {
		return nil
	}
	t, err := r.newTransport(ctx, transport.DOWNLOAD)
	if err != nil {
		return err
	}
	if len(m.objects) != 0 {
		if err := r.batch(ctx, t, m.objects); err != nil {
			return err
		}
		if err := r.odb.Reload(); err != nil {
			return err
		}
	}
	if ignoreLarges {
		return nil
	}
	return r.transfer(ctx, t, m.larges)
}

func (r *Repository) promiseFetch(ctx context.Context, rev string, fetchMissing bool) (oid plumbing.Hash, err error) {
	if oid, err = r.resolveRevision(ctx, rev); err != nil {
		return plumbing.ZeroHash, err
	}
	if r.odb.Exists(oid, true) {
		return oid, nil
	}
	if !fetchMissing {
		return plumbing.ZeroHash, plumbing.NoSuchObject(oid)
	}
	if err := r.fetchAny(ctx, &FetchOptions{
		Target:    oid,
		SizeLimit: NoSizeLimit,
		Deepen:    NoDeepen,
		Depth:     NoDepth,
	}); err != nil {
		return oid, err
	}
	return oid, nil
}

type promiseObject struct {
	oid  plumbing.Hash
	size int64
}

func (o *promiseObject) entry() *odb.Entry {
	return &odb.Entry{Hash: o.oid, Size: o.size}
}

func (r *Repository) promiseMissingFetch(ctx context.Context, o *promiseObject) (err error) {
	t, err := r.newTransport(ctx, transport.DOWNLOAD)
	if err != nil {
		return err
	}
	mode := odb.SINGLE_BAR
	if r.quiet {
		mode = odb.NO_BAR
	}
	if o.size >= r.largeSize() {
		return r.transfer(ctx, t, []*odb.Entry{o.entry()})
	}
	// Fetch missing object
	return r.odb.DoTransfer(ctx, o.oid, func(offset int64) (transport.SizeReader, error) {
		return t.GetObject(ctx, o.oid, offset)
	}, odb.NewSingleBar, mode)
}
