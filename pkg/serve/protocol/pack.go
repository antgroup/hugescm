// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/antgroup/hugescm/modules/binary"
	"github.com/antgroup/hugescm/modules/crc"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve/odb"
)

func writeMetadataHeader(w io.Writer) error {
	if err := binary.Write(w, metaTransportMagic[:], PROTOCOL_VERSION, reserved[:]); err != nil {
		return fmt.Errorf("write metadata magic error: %v", err)
	}
	return nil
}

func writeMetadataItem(w io.Writer, e object.Encoder, oid string) error {
	if e == nil {
		return binary.WriteUint32(w, uint32(0))
	}
	b := streamio.GetBytesBuffer()
	defer streamio.PutBytesBuffer(b)
	if err := e.Encode(b); err != nil {
		return err
	}
	encBytes := b.Bytes()
	if err := binary.WriteUint32(w, uint32(len(encBytes)+plumbing.HASH_HEX_SIZE)); err != nil {
		return err
	}
	if err := binary.Write(w, []byte(oid)); err != nil {
		return err
	}
	n, err := w.Write(encBytes)
	if err != nil {
		return err
	}
	if n != len(encBytes) {
		return fmt.Errorf("failed to write data, tried to write %d bytes, actual %d bytes", len(encBytes), n)
	}
	return nil
}

func WriteBatchObjectsHeader(w io.Writer) error {
	if err := binary.Write(w, objectsTransportMagic[:], PROTOCOL_VERSION, reserved[:]); err != nil {
		return fmt.Errorf("write batch-objects magic error: %v", err)
	}
	return nil
}

func WriteObjectsItem(w io.Writer, r io.Reader, oid string, size int64) error {
	if r == nil {
		return binary.WriteUint32(w, uint32(0)) //END BLOB
	}
	bytesBuffer := streamio.GetByteSlice()
	defer streamio.PutByteSlice(bytesBuffer)
	if err := binary.WriteUint32(w, uint32(size+plumbing.HASH_HEX_SIZE)); err != nil {
		return err
	}
	if err := binary.Write(w, []byte(oid)); err != nil {
		return err
	}
	n, err := io.CopyBuffer(w, r, *bytesBuffer)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("failed to write data, tried to write %d bytes, actual %d bytes", size, n)
	}
	return nil
}

func WriteSingleObjectsHeader(w io.Writer, contentLength, compressedSize int64) error {
	if err := binary.Write(w, objectsTransportMagic[:], PROTOCOL_VERSION, contentLength, compressedSize); err != nil {
		return fmt.Errorf("write object magic error: %v", err)
	}
	return nil
}

type SparseMatcher interface {
	Len() int
	Match(name string) (SparseMatcher, bool)
}

type sparseTreeMatcher struct {
	entries map[string]*sparseTreeMatcher
}

func (m *sparseTreeMatcher) Len() int {
	return len(m.entries)
}

func (m *sparseTreeMatcher) Match(name string) (SparseMatcher, bool) {
	sm, ok := m.entries[name]
	return sm, ok
}

func (m *sparseTreeMatcher) insert(p string) {
	dv := strengthen.StrSplitSkipEmpty(p, '/', 10)
	current := m
	for _, d := range dv {
		e, ok := current.entries[d]
		if !ok {
			e = &sparseTreeMatcher{entries: make(map[string]*sparseTreeMatcher)}
			current.entries[d] = e
		}
		current = e
	}
}

func NewSparseTreeMatcher(dirs []string) SparseMatcher {
	root := &sparseTreeMatcher{entries: make(map[string]*sparseTreeMatcher)}
	for _, d := range dirs {
		root.insert(d)
	}
	return root
}

type Packer struct {
	odb.DB
	crc.Finisher
	w            io.Writer
	count        int
	treeMaxDepth int
	seen         map[plumbing.Hash]bool
	closeFn      func() error
}

// NewPipePacker: SSH protocol
func NewPipePacker(o odb.DB, w io.Writer, treeMaxDepth int, useZSTD bool) (*Packer, error) {
	if treeMaxDepth == -1 {
		treeMaxDepth = math.MaxInt
	}
	var bodyWriter io.Writer
	var closeFn func() error
	switch {
	case useZSTD:
		buffedWriter := streamio.GetBufferWriter(w)
		zstdWriter := streamio.GetZstdWriter(buffedWriter)
		closeFn = func() error {
			streamio.PutZstdWriter(zstdWriter)
			err := buffedWriter.Flush()
			streamio.PutBufferWriter(buffedWriter)
			return err
		}
		bodyWriter = zstdWriter
	default:
		buffedWriter := streamio.GetBufferWriter(w)
		closeFn = func() error {
			err := buffedWriter.Flush()
			streamio.PutBufferWriter(buffedWriter)
			return err
		}
		bodyWriter = buffedWriter
	}
	cw := crc.NewCrc64Writer(bodyWriter)
	p := &Packer{DB: o, w: cw, Finisher: cw, treeMaxDepth: treeMaxDepth, closeFn: closeFn, seen: make(map[plumbing.Hash]bool)}
	if err := writeMetadataHeader(cw); err != nil {
		_ = p.Close()
		return nil, err
	}
	return p, nil
}

func NewHttpPacker(o odb.DB, w http.ResponseWriter, r *http.Request, treeMaxDepth int) (*Packer, error) {
	if treeMaxDepth == -1 {
		treeMaxDepth = math.MaxInt
	}
	var bodyWriter io.Writer
	var closeFn func() error
	switch r.Header.Get("Accept") {
	case ZETA_MIME_COMPRESS_MD:
		buffedWriter := streamio.GetBufferWriter(w)
		zstdWriter := streamio.GetZstdWriter(buffedWriter)
		closeFn = func() error {
			streamio.PutZstdWriter(zstdWriter)
			err := buffedWriter.Flush()
			streamio.PutBufferWriter(buffedWriter)
			return err
		}
		bodyWriter = zstdWriter
		w.Header().Set("Content-Type", ZETA_MIME_COMPRESS_MD)
	case ZETA_MIME_MD:
		fallthrough
	default:
		buffedWriter := streamio.GetBufferWriter(w)
		closeFn = func() error {
			err := buffedWriter.Flush()
			streamio.PutBufferWriter(buffedWriter)
			return err
		}
		bodyWriter = buffedWriter
		w.Header().Set("Content-Type", ZETA_MIME_MD)
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	cw := crc.NewCrc64Writer(bodyWriter)
	p := &Packer{DB: o, w: cw, Finisher: cw, treeMaxDepth: treeMaxDepth, closeFn: closeFn, seen: make(map[plumbing.Hash]bool)}
	if err := writeMetadataHeader(cw); err != nil {
		_ = p.Close()
		return nil, err
	}
	return p, nil
}

func (p *Packer) Close() error {
	if p.closeFn != nil {
		return p.closeFn()
	}
	return nil
}

func (p *Packer) Done() (err error) {
	if err = writeMetadataItem(p.w, nil, ""); err != nil {
		return err
	}
	_, err = p.Finish()
	return err
}

func (p *Packer) WriteAny(ctx context.Context, e object.Encoder, oid string) error {
	return writeMetadataItem(p.w, e, oid)
}

func (p *Packer) WriteDeduplication(ctx context.Context, e object.Encoder, oid plumbing.Hash) error {
	if p.seen[oid] {
		return nil
	}
	p.seen[oid] = true
	return writeMetadataItem(p.w, e, oid.String())
}

func (p *Packer) WriteTree(ctx context.Context, oid plumbing.Hash, depth int) error {
	if depth > p.treeMaxDepth {
		return nil
	}
	if p.seen[oid] {
		return nil
	}
	tree, err := p.Tree(ctx, oid)
	if err != nil {
		return err
	}
	if err := writeMetadataItem(p.w, tree, oid.String()); err != nil {
		return err
	}
	p.count++
	for _, e := range tree.Entries {
		switch e.Type() {
		case object.TreeObject:
			if err := p.WriteTree(ctx, e.Hash, depth+1); err != nil {
				return err
			}
		case object.FragmentsObject:
			if !p.seen[e.Hash] {
				ff, err := p.Fragments(ctx, e.Hash)
				if err != nil {
					return err
				}
				if err := writeMetadataItem(p.w, ff, ff.Hash.String()); err != nil {
					return err
				}
				p.count++
				p.seen[e.Hash] = true
			}
		default:
			// nothing
		}
	}
	p.seen[oid] = true
	return nil
}

func (p *Packer) WriteSparseTree(ctx context.Context, oid plumbing.Hash, m SparseMatcher, depth int) error {
	if depth > p.treeMaxDepth {
		return nil
	}
	if m == nil || m.Len() == 0 {
		return p.WriteTree(ctx, oid, depth+1)
	}
	if p.seen[oid] {
		return nil
	}
	tree, err := p.Tree(ctx, oid)
	if err != nil {
		return err
	}
	if err := writeMetadataItem(p.w, tree, oid.String()); err != nil {
		return err
	}
	p.count++
	for _, e := range tree.Entries {
		switch e.Type() {
		case object.TreeObject:
			if sub, ok := m.Match(e.Name); ok {
				if err := p.WriteSparseTree(ctx, e.Hash, sub, depth+1); err != nil {
					return err
				}
			}
		case object.FragmentsObject:
			if !p.seen[e.Hash] {
				ff, err := p.Fragments(ctx, e.Hash)
				if err != nil {
					return err
				}
				if err := writeMetadataItem(p.w, ff, ff.Hash.String()); err != nil {
					return err
				}
				p.count++
				p.seen[e.Hash] = true
			}
		default:
			// nothing
		}
	}
	p.seen[oid] = true
	return nil
}

func (p *Packer) WriteDeepenMetadata(ctx context.Context, current *object.Commit, deepenFrom, have plumbing.Hash, deepen int) error {
	if deepen == -1 {
		deepen = math.MaxInt
	}
	iter := object.NewCommitIterBSF(current, nil, nil)
	defer iter.Close()
	for range deepen {
		cc, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		oid := cc.Hash
		if oid == deepenFrom || oid == have {
			break
		}
		if err := writeMetadataItem(p.w, cc, oid.String()); err != nil {
			return err
		}
		if err := p.WriteTree(ctx, cc.Tree, 0); err != nil {
			return err
		}
	}
	return nil
}

func (p *Packer) WriteDeepenSparseMetadata(ctx context.Context, current *object.Commit, deepenFrom, have plumbing.Hash, deepen int, paths []string) error {
	if deepen == -1 {
		deepen = math.MaxInt
	}
	m := NewSparseTreeMatcher(paths)
	iter := object.NewCommitIterBSF(current, nil, nil)
	defer iter.Close()
	for range deepen {
		cc, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		oid := cc.Hash
		if oid == deepenFrom || oid == have {
			break
		}
		if err := writeMetadataItem(p.w, cc, oid.String()); err != nil {
			return err
		}

		if err := p.WriteSparseTree(ctx, cc.Tree, m, 0); err != nil {
			return err
		}
	}
	return nil
}
