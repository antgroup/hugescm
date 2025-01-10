// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/progressbar"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/progress"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/transport"
)

type ProgressMode int

const (
	NO_BAR ProgressMode = iota
	SINGLE_BAR
	MULTI_BARS
)

type MakeBar func(r io.Reader, total int64, current int64, oid plumbing.Hash, round int) (io.Reader, io.Closer)

func NewSingleBar(r io.Reader, total int64, current int64, oid plumbing.Hash, round int) (io.Reader, io.Closer) {
	var desc string
	if round == 0 {
		desc = fmt.Sprintf("%s %s ...", tr.W("Downloading"), oid.String()[:8])
	} else {
		desc = fmt.Sprintf("%s %s [\x1b[33m%s\x1b[0m] ...", tr.W("Downloading"), oid.String()[:8], tr.W("retrying"))
	}
	bar := progressbar.NewOptions64(
		total,
		progressbar.OptionSeekTo(current),
		progressbar.OptionShowBytes(true),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionSetDescription(desc),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetTheme(progress.MakeTheme()))
	return io.TeeReader(r, bar), bar
}

type Transfer func(offset int64) (transport.SizeReader, error)

func checkClose(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}

func (d *ODB) doTransfer(ctx context.Context, oid plumbing.Hash, fd *os.File, transfer Transfer, round int, m MakeBar, mode ProgressMode) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	si, err := fd.Stat()
	if err != nil {
		return err
	}
	offset := si.Size()
	sr, err := transfer(offset)
	if err != nil {
		return err
	}
	offsetBytes := sr.Offset()
	if offsetBytes != 0 && mode == SINGLE_BAR {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rServer accepted resume download request: %s from byte %d\n", oid, offset)
	}
	if _, err = fd.Seek(offsetBytes, io.SeekStart); err != nil {
		_ = sr.Close()
		return err
	}
	if offsetBytes != offset {
		if err = fd.Truncate(offsetBytes); err != nil {
			_ = sr.Close()
			return err
		}
	}
	var r io.Reader = sr
	var mc io.Closer
	if mode != NO_BAR {
		r, mc = m(sr, sr.Size(), sr.Offset(), oid, round)
	}
	if _, err = fd.ReadFrom(r); err != nil {
		checkClose(mc)
		_ = sr.Close()
		if errors.Is(err, io.EOF) && sr.LastError() != nil {
			return sr.LastError()
		}
		return err
	}
	checkClose(mc)
	_ = sr.Close()
	return nil
}

// FIXME: In Windows, truncating a file may fail due to security software or kernel file locking.
func (d *ODB) doTransferFallback(ctx context.Context, oid plumbing.Hash, transfer Transfer, m MakeBar, mode ProgressMode) error {
	start := time.Now()
	fd, err := d.NewTruncateFD(oid)
	if err != nil {
		return err
	}
	if err = d.doTransfer(ctx, oid, fd, transfer, 0, m, mode); err != nil {
		_ = fd.Close()
		return err
	}
	si, err := fd.Stat()
	if err != nil {
		_ = fd.Close()
		return err
	}
	if err := d.ValidateFD(fd, oid); err != nil {
		return err
	}
	if mode == SINGLE_BAR {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rDownload %s completed, size: %s %s: %v\n", oid, strengthen.HumanateSize(si.Size()), tr.W("time spent"), time.Since(start).Truncate(time.Millisecond))
	}
	return nil
}

func (d *ODB) DoTransfer(ctx context.Context, oid plumbing.Hash, transfer Transfer, m MakeBar, mode ProgressMode) error {
	start := time.Now()
	fd, err := d.NewFD(oid)
	if err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if err = d.doTransfer(ctx, oid, fd, transfer, i, m, mode); err == nil {
			break
		}
		if os.IsPermission(err) {
			_ = fd.Close()
			return d.doTransferFallback(ctx, oid, transfer, m, mode)
		}
		if err != io.ErrUnexpectedEOF {
			_ = fd.Close()
			return err
		}
	}
	if err != nil {
		_ = fd.Close()
		return err
	}
	si, err := fd.Stat()
	if err != nil {
		_ = fd.Close()
		return err
	}
	if err := d.ValidateFD(fd, oid); err != nil {
		return err
	}
	if mode == SINGLE_BAR {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rDownload %s completed, size: %s %s: %v\n", oid, strengthen.HumanateSize(si.Size()), tr.W("time spent"), time.Since(start).Truncate(time.Millisecond))
	}
	return nil
}
