// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sync"
	"time"

	"github.com/antgroup/hugescm/modules/oss"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"golang.org/x/sync/errgroup"
)

const (
	zetaBlobMIME           = "application/vnd.zeta-blob"
	MiByte           int64 = 1048576
	defaultThreshold       = 100 * MiByte
)

func ossJoin(rid int64, oid plumbing.Hash) string {
	h := oid.String()
	return fmt.Sprintf("zeta/%03d/%d/%s/%s/%s", rid%1000, rid, h[0:2], h[2:4], h)
}

func (o *ODB) ossExists(ctx context.Context, oid plumbing.Hash) error {
	_, err := o.bucket.Stat(ctx, ossJoin(o.rid, oid))
	if errors.Is(err, os.ErrNotExist) {
		return plumbing.NoSuchObject(oid)
	}
	return err
}

func (o *ODB) Stat(ctx context.Context, oid plumbing.Hash) (*oss.Stat, error) {
	return o.bucket.Stat(ctx, ossJoin(o.rid, oid))
}

type Representation struct {
	Href      string
	Size      int64
	ExpiresAt int64
}

func (o *ODB) Share(ctx context.Context, oid plumbing.Hash, expiresAt int64) (*Representation, error) {
	resourcePath := ossJoin(o.rid, oid)
	si, err := o.bucket.Stat(ctx, resourcePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, plumbing.NoSuchObject(oid)
		}
		return nil, err
	}
	href := o.bucket.Share(ctx, resourcePath, expiresAt)
	return &Representation{
		Href:      href,
		Size:      si.Size,
		ExpiresAt: expiresAt,
	}, nil
}

// Store directly to OSS
// Verify and upload to OSS
// Typically used for uploading larger binary files.
func (o *ODB) WriteDirect(ctx context.Context, oid plumbing.Hash, r io.Reader, size int64) (int64, error) {
	resourcePath := ossJoin(o.rid, oid)
	si, err := o.bucket.Stat(ctx, resourcePath)
	if err == nil {
		return si.Size, nil
	}
	if !os.IsNotExist(err) {
		return 0, err
	}
	pr, pw := io.Pipe()

	var got plumbing.Hash

	g, newCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if got, err = object.HashFrom(pr); err != nil {
			return err
		}
		return nil
	})
	g.Go(func() error {
		defer pw.Close()
		if size > 0 {
			if err := o.bucket.LinearUpload(newCtx, resourcePath, io.TeeReader(r, pw), size, zetaBlobMIME); err != nil {
				pw.CloseWithError(err)
				return err
			}
		} else {
			if err := o.bucket.Put(newCtx, resourcePath, io.TeeReader(r, pw), zetaBlobMIME); err != nil {
				pw.CloseWithError(err)
				return err
			}
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return 0, err
	}
	if got != oid {
		cleanupCtx, cancelCtx := context.WithTimeout(context.Background(), time.Minute)
		defer cancelCtx()
		_ = o.bucket.Delete(cleanupCtx, resourcePath)
		return 0, fmt.Errorf("unexpected blob oid got '%s' want '%s'", got, oid)
	}
	return size, nil
}

func (o *ODB) Push(ctx context.Context, oid plumbing.Hash) error {
	resourcePath := ossJoin(o.rid, oid)
	if _, err := o.bucket.Stat(ctx, resourcePath); err == nil {
		return nil
	}
	sr, err := o.odb.SizeReader(oid, false)
	if err != nil {
		return err
	}
	defer sr.Close()

	if err := o.bucket.LinearUpload(ctx, resourcePath, sr, sr.Size(), zetaBlobMIME); err != nil {
		return err
	}
	o.cdb.Mark(o.rid, oid)
	return nil
}

type uploadGroup struct {
	ch     chan plumbing.Hash
	errors chan error
	wg     sync.WaitGroup
}

func (g *uploadGroup) waitClose() {
	close(g.ch)
	g.wg.Wait()
}

func (g *uploadGroup) submit(ctx context.Context, oid plumbing.Hash) error {
	// In case the context has been cancelled, we have a race between observing an error from
	// the killed Git process and observing the context cancellation itself. But if we end up
	// here because of cancellation of the Git process, we don't want to pass that one down the
	// pipeline but instead just stop the pipeline gracefully. We thus have this check here up
	// front to error messages from the Git process.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-g.errors:
		return err
	default:
	}

	select {
	case g.ch <- oid:
		return nil
	case err := <-g.errors:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *uploadGroup) upload(ctx context.Context, o *ODB) error {
	for oid := range g.ch {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}
		if err := o.Push(ctx, oid); err != nil {
			return err
		}
	}
	return nil
}

func (g *uploadGroup) run(ctx context.Context, o *ODB) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		err := g.upload(ctx, o)
		g.errors <- err
	}()
}

// BatchObjects: batch upload objects
func (o *ODB) BatchObjects(ctx context.Context, oids []plumbing.Hash, batchLimit int) error {
	if len(oids) == 0 {
		return nil
	}
	g := &uploadGroup{
		ch:     make(chan plumbing.Hash, batchLimit),
		errors: make(chan error, batchLimit),
	}

	newCtx, cancelCtx := context.WithCancelCause(ctx)
	defer cancelCtx(nil)
	for range batchLimit {
		g.run(newCtx, o)
	}
	for _, oid := range oids {
		if err := g.submit(ctx, oid); err != nil {
			g.waitClose()
			return err
		}
	}
	g.waitClose()
	close(g.errors)
	for err := range g.errors {
		if err != nil {
			return err
		}
	}
	return nil
}

func OssRemoveFiles(ctx context.Context, b oss.Bucket, rid int64) error {
	prefix := fmt.Sprintf("zeta/%03d/%d/", rid%1000, rid)
	var continuationToken string
	for {
		objects, nextContinuationToken, err := b.ListObjects(ctx, prefix, continuationToken)
		if err != nil {
			return err
		}
		continuationToken = nextContinuationToken
		objectKeys := make([]string, 0, len(objects))
		for _, o := range objects {
			objectKeys = append(objectKeys, o.Key)
		}
		if err := b.DeleteMultipleObjects(ctx, objectKeys); err != nil {
			return err
		}
		if len(continuationToken) == 0 {
			break
		}
	}
	return nil
}

type LargeObject struct {
	OID            string `json:"oid"`
	CompressedSize int64  `json:"compressed_size"`
}

type StatObjectsResult struct {
	Larges  []*LargeObject `json:"larges,omitempty"`
	Objects int            `json:"count"`
	Size    int64          `json:"size"`
}

func StatObjects(ctx context.Context, b oss.Bucket, rid int64, threshold int64) (*StatObjectsResult, error) {
	prefix := fmt.Sprintf("zeta/%03d/%d/", rid%1000, rid)
	var result StatObjectsResult
	var continuationToken string
	if threshold == 0 {
		threshold = defaultThreshold
	}
	if threshold == -1 {
		threshold = math.MaxInt64
	}
	for {
		objects, nextContinuationToken, err := b.ListObjects(ctx, prefix, continuationToken)
		if err != nil {
			return nil, err
		}
		continuationToken = nextContinuationToken
		result.Objects += len(objects)
		for _, o := range objects {
			if o.Size > threshold {
				result.Larges = append(result.Larges, &LargeObject{OID: path.Base(o.Key), CompressedSize: o.Size})
			}
			result.Size += o.Size
		}
		if len(continuationToken) == 0 {
			break
		}
	}
	return &result, nil
}
