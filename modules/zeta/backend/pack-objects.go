// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend/pack"
	"github.com/antgroup/hugescm/modules/zeta/backend/storage"
)

type Indicators interface {
	Add(n int)
	Wait()
	Run(ctx context.Context)
}

type NewIndicators func(description, completed string, total uint64, quiet bool) Indicators

type nonIndicators struct{}

func (p nonIndicators) Add(n int)               {}
func (p nonIndicators) Wait()                   {}
func (p nonIndicators) Run(ctx context.Context) {}

var (
	_ Indicators = &nonIndicators{}
)

func preservePack(root, quarantine string) error {
	packDir := filepath.Join(root, "pack")
	if err := mkdir(packDir); err != nil {
		return err
	}
	dirs, err := os.ReadDir(quarantine)
	if err != nil {
		return err
	}
	for _, d := range dirs {
		if d.IsDir() {
			continue
		}
		if err := finalizeObject(filepath.Join(quarantine, d.Name()), filepath.Join(packDir, d.Name())); err != nil {
			return err
		}
	}
	return nil
}

type packedObject struct {
	size         int64
	modification int64
	packed       bool
}

type packedObjects map[plumbing.Hash]*packedObject

func openObject(ro storage.Storage, oid plumbing.Hash, o *packedObject) (SizeReader, int64, error) {
	rc, err := ro.Open(oid)
	if err != nil {
		return nil, 0, err
	}
	switch v := rc.(type) {
	case *os.File:
		si, err := v.Stat()
		if err != nil {
			v.Close()
			return nil, 0, err
		}
		return &sizeReader{Reader: v, closer: v, size: si.Size()}, o.modification, nil
	case *pack.SizeReader:
		return &sizeReader{Reader: v, closer: v, size: v.Size()}, o.modification, nil
	default:
	}
	_ = rc.Close()
	return nil, 0, errors.New("unable detect reader size")
}

func repackMetadataObjects(ctx context.Context, ro storage.Storage, objects packedObjects, quarantine string, bar Indicators) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	w, err := pack.NewWriter(quarantine, uint32(len(objects)))
	if err != nil {
		return err
	}
	defer w.Close()
	for oid, po := range objects {
		bar.Add(1)
		sr, modification, err := openObject(ro, oid, po)
		if err != nil {
			return err
		}
		err = w.Write(oid, uint32(sr.Size()), sr, modification)
		sr.Close()
		if err != nil {
			return err
		}
	}
	return w.WriteTrailer()
}

func repackBlobObjects(ctx context.Context, opts *PackOptions, ro storage.Storage, fo *fileStorer, objects packedObjects, quarantine string, bar Indicators) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	unpack := func(oid plumbing.Hash, po *packedObject, sr SizeReader) error {
		defer sr.Close()
		if !po.packed {
			return nil
		}
		return fo.Unpack(oid, sr)
	}
	w, err := pack.NewWriter(quarantine, 0)
	if err != nil {
		return err
	}
	defer w.Close()
	for oid, po := range objects {
		bar.Add(1)
		sr, modification, err := openObject(ro, oid, po)
		if err != nil {
			return err
		}
		if sr.Size() > opts.PackThreshold {
			if err := unpack(oid, po, sr); err != nil {
				return err
			}
			objects[oid] = nil
			continue
		}
		err = w.Write(oid, uint32(sr.Size()), sr, modification)
		sr.Close()
		if err != nil {
			return err
		}
	}
	return w.WriteTrailer()
}

func repackObjects(ctx context.Context, opts *PackOptions, ro storage.Storage, fo *fileStorer, objects packedObjects, quarantine string, meta bool) (err error) {
	bar := opts.NewIndicators("Writing objects", "", uint64(len(objects)), opts.Quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)

	if meta {
		err = repackMetadataObjects(ctx, ro, objects, quarantine, bar)
	} else {
		err = repackBlobObjects(ctx, opts, ro, fo, objects, quarantine, bar)
	}
	if err != nil {
		cancelCtx(err)
		bar.Wait()
		return err
	}
	cancelCtx(nil)
	bar.Wait()
	return nil
}

func pruneObjects0(ctx context.Context, fo *fileStorer, objects packedObjects, bar Indicators) int {
	var count int
	for oid, po := range objects {
		bar.Add(1)
		if po == nil {
			continue
		}
		if err := fo.PruneObject(ctx, oid); err == context.Canceled {
			break
		}
		count++
	}
	return count
}

func pruneObjects(ctx context.Context, opts *PackOptions, fo *fileStorer, objects packedObjects) int {
	bar := opts.NewIndicators("Prune objects", "", uint64(len(objects)), opts.Quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	bar.Run(newCtx)
	count := pruneObjects0(ctx, fo, objects, bar)
	cancelCtx(nil)
	bar.Wait()
	return count
}

func packObjectsInternal(ctx context.Context, opts *PackOptions, root string, meta bool) error {
	fsobj := newFileStorer(root, "", opts.CompressionALGO)
	packs, err := pack.NewScanner(root)
	if err != nil {
		return fmt.Errorf("new scanner error: %w", err)
	}
	ro := storage.MultiStorage(fsobj, packs)
	closed := false
	defer func() {
		if !closed {
			ro.Close()
		}
	}()
	objects := make(packedObjects)
	looseObjects, err := fsobj.looseObjects(opts.PackThreshold)
	if err != nil {
		return err
	}
	step := "blob"
	if meta {
		step = "metadata"
	}

	if len(looseObjects) == 0 {
		// no small loose objects, skipped.
		opts.Printf("Pack %s objects: no smaller loose object, skipping packing.\n", step)
		return nil
	}
	for _, o := range looseObjects {
		objects[o.Hash] = &packedObject{size: o.Size, modification: o.Modification}
	}
	var packedEntries int
	err = packs.PackedObjects(func(oid plumbing.Hash, modification int64) error {
		objects[oid] = &packedObject{modification: modification, packed: true}
		packedEntries++
		return nil
	})
	if err != nil {
		return err
	}
	quarantineDir, err := os.MkdirTemp(root, "quarantine-")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(quarantineDir)
	}()

	opts.Printf("Pack %s objects: loose object %d packed objects %d\n", step, len(looseObjects), packedEntries)
	if err := repackObjects(ctx, opts, ro, fsobj, objects, quarantineDir, meta); err != nil {
		return fmt.Errorf("repack objects [metadata: %v] %w", meta, err)
	}
	if err := preservePack(root, quarantineDir); err != nil {
		return err
	}
	names := packs.Names()
	_ = ro.Close()
	closed = true
	for _, p := range names {
		_ = os.Remove(p)                                          // PACK
		_ = os.Remove(strings.TrimSuffix(p, ".pack") + ".idx")    // PACK INDEX
		_ = os.Remove(strings.TrimSuffix(p, ".pack") + ".mtimes") // PACK INDEX
	}
	count := pruneObjects(ctx, opts, fsobj, objects)
	var prunedDirs int
	if prunedDirs, err = fsobj.Prune(ctx); err != nil {
		return err
	}
	opts.Printf("Removed duplicate packages: %d, duplicate objects: %d empty dirs: %d\n", len(names), count, prunedDirs)
	return nil
}

type PackOptions struct {
	ZetaDir         string
	SharingRoot     string
	Quiet           bool
	CompressionALGO string
	PackThreshold   int64
	Logger          func(format string, a ...any)
	NewIndicators   NewIndicators
}

const (
	DefaultPackThreshold = 50 * 1024 * 1024 // 50M
)

func (opts *PackOptions) checkInit() {
	if opts.PackThreshold == 0 {
		opts.PackThreshold = DefaultPackThreshold
	}
	if opts.CompressionALGO == "" {
		opts.CompressionALGO = "zstd"
	}
	if opts.NewIndicators == nil {
		opts.NewIndicators = func(description, completed string, total uint64, quiet bool) Indicators {
			return &nonIndicators{}
		}
	}
}

func (opts *PackOptions) Printf(format string, a ...any) {
	if opts.Logger != nil {
		opts.Logger(format, a...)
	}
}

func PackObjects(ctx context.Context, opts *PackOptions) error {
	opts.checkInit()
	metaRoot := filepath.Join(opts.ZetaDir, "metadata")
	if err := packObjectsInternal(ctx, opts, metaRoot, true); err != nil {
		return err
	}
	root := filepath.Join(opts.ZetaDir, "blob")
	if len(opts.SharingRoot) != 0 {
		root = filepath.Join(opts.SharingRoot, "blob")
	}
	return packObjectsInternal(ctx, opts, root, false)
}
