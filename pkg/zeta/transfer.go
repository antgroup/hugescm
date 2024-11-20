// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/config"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/transport/http"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/term"
)

// termWidth function returns the visible width of the current terminal
// and can be redefined for testing
var termWidth = func() (width int, err error) {
	width, _, err = term.GetSize(int(os.Stderr.Fd()))
	if err == nil {
		return width, nil
	}
	return 0, err
}

const (
	largeSize = 10 << 20 // 10M
)

func (r *Repository) getLinks(ctx context.Context, t transport.Transport, larges []*odb.Entry) ([]*transport.Representation, error) {
	wantObjects := make([]*transport.WantObject, 0, len(larges))
	for _, o := range larges {
		if r.odb.Exists(o.Hash, false) {
			continue
		}
		wantObjects = append(wantObjects, &transport.WantObject{OID: o.Hash.String()})
	}
	if len(wantObjects) == 0 {
		return nil, nil
	}
	objects, err := t.Shared(ctx, wantObjects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "batch shared response error: %v\n", err)
		return nil, err
	}
	return objects, nil
}

func (r *Repository) directMultiTransferQuiet(ctx context.Context, t http.Downloader, objects []*transport.Representation) error {
	wg := &sync.WaitGroup{}
	errs := make(chan error, len(objects))
	for _, e := range objects {
		wg.Add(1)
		go func(ctx context.Context, o *transport.Representation) {
			defer wg.Done()
			if o.IsExpired() {
				fmt.Fprintf(os.Stderr, "object '%s' download link expired at: %v\n", o.OID, o.ExpiresAt)
				errs <- nil
				return
			}
			oid := plumbing.NewHash(o.OID)
			if err := r.odb.DoTransfer(ctx, oid,
				func(offset int64) (transport.SizeReader, error) {
					return t.Download(ctx, o, offset)
				},
				nil, odb.NO_BAR); err != nil {
				errs <- fmt.Errorf("download %s error: %w", oid, err)
				return
			}
			errs <- nil
		}(ctx, e.Copy())
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) directMultiTransfer(ctx context.Context, t http.Downloader, objects []*transport.Representation) error {
	if r.quiet {
		return r.directMultiTransferQuiet(ctx, t, objects)
	}
	width, err := termWidth()
	if err != nil {
		width = 80
	}
	if width > 80 {
		width = 80
	}
	p := mpb.New(
		mpb.WithOutput(os.Stderr),
		mpb.WithAutoRefresh(),
		mpb.WithWidth(width),
	)
	style := mpb.BarStyle()
	style.Filler("\x1b[38;2;72;198;239m#\x1b[0m")
	errs := make(chan error, len(objects))
	for _, e := range objects {
		oid := plumbing.NewHash(e.OID)
		task := fmt.Sprintf("%s %s", W("Downloading"), shortHash(oid))
		bar := p.New(-1,
			mpb.BarStyle().Filler("#").Padding(" "),
			mpb.PrependDecorators(
				decor.Name(task, decor.WC{W: len(task), C: decor.DindentRight}),
				decor.Total(decor.SizeB1024(0), "% .2f", decor.WCSyncWidth),
			),
			mpb.BarWidth(width),
			mpb.AppendDecorators(
				decor.EwmaSpeed(decor.SizeB1024(0), "% .2f ", 90),
				decor.OnComplete(
					// ETA decorator with ewma age of 30
					decor.EwmaETA(decor.ET_STYLE_GO, 30), "done",
				),
			),
		)
		go func(ctx context.Context, o *transport.Representation, bar *mpb.Bar) {
			if o.IsExpired() {
				fmt.Fprintf(os.Stderr, "object '%s' download link expired at: %v\n", o.OID, o.ExpiresAt)
				bar.SetTotal(-1, true)
				errs <- nil
				return
			}
			if err := r.odb.DoTransfer(ctx, oid,
				func(offset int64) (transport.SizeReader, error) {
					return t.Download(ctx, o, offset)
				},
				func(reader io.Reader, total, current int64, oid plumbing.Hash, round int) (io.Reader, io.Closer) {
					bar.SetTotal(total, false)
					bar.SetCurrent(current)
					rc := bar.ProxyReader(reader)
					return rc, rc
				}, odb.MULTI_BARS); err != nil {
				bar.Abort(true)
				errs <- fmt.Errorf("download %s error: %w", oid, err)
				return
			}
			bar.SetTotal(-1, true)
			errs <- nil
		}(ctx, e.Copy(), bar)
	}
	p.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) directGet(ctx context.Context, objects []*transport.Representation) error {
	t := http.NewDownloader(r.verbose, parseInsecureSkipTLS(r.Config, r.values))
	concurrent := r.ConcurrentTransfers()
	r.DbgPrint("concurrent transfers %d", concurrent)
	if concurrent <= 1 || len(objects) == 1 {
		mode := odb.SINGLE_BAR
		if r.quiet {
			mode = odb.NO_BAR
		}
		for _, e := range objects {
			if e.IsExpired() {
				fmt.Fprintf(os.Stderr, "object '%s' download link expired at: %v\n", e.OID, e.ExpiresAt)
				return nil
			}
			oid := plumbing.NewHash(e.OID)
			if err := r.odb.DoTransfer(ctx, oid,
				func(fromBytes int64) (transport.SizeReader, error) {
					return t.Download(ctx, e, fromBytes)
				},
				odb.NewSingleBar, mode); err != nil {
				return err
			}
		}
		return nil
	}
	for len(objects) > 0 {
		g := min(concurrent, len(objects))
		if err := r.directMultiTransfer(ctx, t, objects[:g]); err != nil {
			return err
		}
		objects = objects[g:]
	}
	return nil
}

func (r *Repository) multiTransferQuiet(ctx context.Context, t transport.Transport, larges []*odb.Entry) error {
	wg := &sync.WaitGroup{}
	errs := make(chan error, len(larges))
	for _, e := range larges {
		wg.Add(1)
		go func(ctx context.Context, oid plumbing.Hash) {
			defer wg.Done()
			if err := r.odb.DoTransfer(ctx, oid,
				func(offset int64) (transport.SizeReader, error) {
					return t.GetObject(ctx, oid, offset)
				},
				nil, odb.NO_BAR); err != nil {
				errs <- fmt.Errorf("download %s error: %w", oid, err)
				return
			}
			errs <- nil
		}(ctx, e.Hash)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) multiTransfer(ctx context.Context, t transport.Transport, larges []*odb.Entry) error {
	if r.quiet {
		return r.multiTransferQuiet(ctx, t, larges)
	}
	width, err := termWidth()
	if err != nil {
		width = 80
	}
	if width > 80 {
		width = 80
	}
	p := mpb.New(
		mpb.WithOutput(os.Stderr),
		mpb.WithAutoRefresh(),
		mpb.WithWidth(width),
	)
	style := mpb.BarStyle()
	style.Filler("\x1b[38;2;72;198;239m#\x1b[0m")
	errs := make(chan error, len(larges))
	for _, o := range larges {
		task := fmt.Sprintf("%s %s", W("Downloading"), shortHash(o.Hash))
		bar := p.New(-1,
			mpb.BarStyle().Filler("#").Padding(" "),
			mpb.PrependDecorators(
				decor.Name(task, decor.WC{W: len(task), C: decor.DindentRight}),
				decor.Total(decor.SizeB1024(0), "% .2f", decor.WCSyncWidth),
			),
			mpb.BarWidth(width),
			mpb.AppendDecorators(
				decor.EwmaSpeed(decor.SizeB1024(0), "% .2f ", 90),
				decor.OnComplete(
					// ETA decorator with ewma age of 30
					decor.EwmaETA(decor.ET_STYLE_GO, 30), "done",
				),
			),
		)
		go func(ctx context.Context, oid plumbing.Hash, bar *mpb.Bar) {
			if err := r.odb.DoTransfer(ctx, oid,
				func(fromBytes int64) (transport.SizeReader, error) {
					return t.GetObject(ctx, oid, fromBytes)
				},
				func(reader io.Reader, total, current int64, oid plumbing.Hash, round int) (io.Reader, io.Closer) {
					bar.SetTotal(total, false)
					bar.SetCurrent(current)
					rc := bar.ProxyReader(reader)
					return rc, rc
				}, odb.MULTI_BARS); err != nil {
				bar.Abort(true)
				errs <- fmt.Errorf("download %s error: %w", oid, err)
				return
			}
			bar.SetTotal(-1, true)
			errs <- nil
		}(ctx, o.Hash, bar)
	}
	p.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) transfer(ctx context.Context, t transport.Transport, larges []*odb.Entry) error {
	if len(larges) == 0 {
		return nil
	}
	accelerator := map[config.Accelerator]func(context.Context, []*transport.Representation) error{
		config.Direct:    r.directGet,
		config.Aria2:     r.aria2Get,
		config.Dragonfly: r.dragonflyGet,
	}
	if h, ok := accelerator[r.Accelerator()]; ok {
		for i := 0; i < 3; i++ {
			objects, err := r.getLinks(ctx, t, larges)
			if err != nil {
				return err
			}
			if len(objects) == 0 {
				return nil
			}
			if err := h(ctx, objects); err != nil {
				return err
			}
		}
		return errors.New("download large files failed")
	}
	concurrent := r.ConcurrentTransfers()
	r.DbgPrint("concurrent transfers %d", concurrent)
	if concurrent <= 1 || len(larges) == 1 {
		mode := odb.SINGLE_BAR
		if r.quiet {
			mode = odb.NO_BAR
		}
		for _, e := range larges {
			if err := r.odb.DoTransfer(ctx, e.Hash,
				func(offset int64) (transport.SizeReader, error) {
					return t.GetObject(ctx, e.Hash, offset)
				},
				odb.NewSingleBar, mode); err != nil {
				return err
			}
		}
		return nil
	}
	for len(larges) > 0 {
		g := min(concurrent, len(larges))
		if err := r.multiTransfer(ctx, t, larges[:g]); err != nil {
			return err
		}
		larges = larges[g:]
	}
	return nil
}
