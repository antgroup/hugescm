// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/binary"
	"github.com/antgroup/hugescm/modules/crc"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/format/pktline"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/pkg/progress"
)

var (
	PUSH_STREAM_MAGIC        = [4]byte{'Z', 'P', '\x00', '\x01'}
	supportedVersion  uint32 = 1
	reserved          [16]byte
)

func (d *ODB) writeObjectToPack(oid plumbing.Hash, metadata bool, w io.Writer, r io.Reader, size int64) error {
	sizeU := uint64(size + plumbing.HASH_HEX_SIZE)
	if metadata {
		sizeU = uint64(-sizeU)
	}
	if err := binary.WriteUint64(w, sizeU); err != nil {
		return err
	}
	oids := oid.String()
	if err := binary.Write(w, []byte(oids)); err != nil {
		return err
	}
	n, err := io.Copy(w, r)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("expected to write pack %d bytes, actually wrote %d bytes", size, n)
	}
	return nil
}

type HaveObject struct {
	Hash plumbing.Hash
	Size int64
}

type PushObjects struct {
	Metadata     []plumbing.Hash
	Objects      []plumbing.Hash
	LargeObjects []*HaveObject
}

type Indicators interface {
	Add(int)
}

func (d *ODB) onPush(ctx context.Context, originWriter io.Writer, objects *PushObjects, ia Indicators) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	w := crc.NewCrc64Writer(originWriter)
	if err := binary.Write(w, PUSH_STREAM_MAGIC[:]); err != nil {
		return err
	}
	if err := binary.WriteUint32(w, supportedVersion); err != nil {
		return err
	}
	if err := binary.Write(w, reserved[:]); err != nil {
		return err
	}
	for _, oid := range objects.Metadata {
		sr, err := d.SizeReader(oid, true)
		if err != nil {
			return err
		}
		if err := d.writeObjectToPack(oid, true, w, sr, sr.Size()); err != nil {
			return err
		}
		ia.Add(1)
	}
	for _, oid := range objects.Objects {
		if oid == backend.BLANK_BLOB_HASH {
			continue
		}
		sr, err := d.SizeReader(oid, false)
		if plumbing.IsNoSuchObject(err) {
			// NOTE: ignore no such object
			ia.Add(1)
			continue
		}
		if err != nil {
			return err
		}
		if err := d.writeObjectToPack(oid, false, w, sr, sr.Size()); err != nil {
			return err
		}
		ia.Add(1)
	}
	// END: 0 length object
	if err := binary.WriteUint64(w, 0); err != nil {
		return err
	}
	if _, err := w.Finish(); err != nil {
		return err
	}
	return nil
}

func (d *ODB) PushTo(ctx context.Context, originWriter io.Writer, objects *PushObjects, quiet bool) error {
	ia := progress.NewIndicators("Push objects", "Push objects completed", 0, quiet)
	newCtx, cancelCtx := context.WithCancelCause(ctx)
	ia.Run(newCtx)
	if err := d.onPush(ctx, originWriter, objects, ia); err != nil {
		cancelCtx(err)
		ia.Wait()
		return err
	}
	cancelCtx(nil)
	ia.Wait()
	return nil
}

type Report struct {
	ReferenceName plumbing.ReferenceName
	NewRev        string
	Rejected      bool
	Reason        string
}

func sanitizeLine(s string) string {
	p := strings.IndexByte(s, '\n')
	if p != -1 {
		s = s[:p]
	}
	return term.SanitizeANSI(strings.TrimSpace(s), term.StderrLevel != term.LevelNone)
}

func (d *ODB) OnReport(ctx context.Context, refname plumbing.ReferenceName, reader io.Reader) (result *Report, err error) {
	var b strings.Builder
	r := pktline.NewScanner(io.TeeReader(reader, &b))
	var newLine bool
	defer func() {
		if newLine {
			fmt.Fprintf(os.Stderr, "\n")
		}
	}()
	for r.Scan() {
		line := string(r.Bytes())
		pos := strings.IndexByte(line, ' ')
		if pos == -1 {
			return nil, fmt.Errorf("bad report line: %s", sanitizeLine(line))
		}
		lab := line[0:pos]
		substr := line[pos+1:]
		if lab == "rate" {
			fmt.Fprintf(os.Stderr, "\x1b[2K\rremote: %s", sanitizeLine(substr))
			newLine = true
			continue
		}
		if newLine {
			// newLine fill
			_, _ = os.Stderr.WriteString("\n")
			newLine = false
		}
		if lab == "unpack" {
			if substr != "ok" {
				_, _ = term.SanitizedF("remote: unpack %s\n", substr)
				result = &Report{ReferenceName: refname, Reason: substr, Rejected: true}
				break
			}
			fmt.Fprintf(os.Stderr, "remote: unpack success\n")
			continue
		}
		if lab == "ok" {
			refname, newRev, _ := strings.Cut(line[pos+1:], " ")
			if !plumbing.ValidateReferenceName([]byte(refname)) {
				return nil, fmt.Errorf("remote: invalid refname '%s' for ok-result", sanitizeLine(refname))
			}
			if !plumbing.ValidateHashHex(newRev) {
				return nil, fmt.Errorf("remote: invalid new-rev '%s' for ok-result", sanitizeLine(refname))
			}
			result = &Report{ReferenceName: plumbing.ReferenceName(refname), NewRev: newRev}
			break
		}
		if lab == "ng" {
			pos = strings.IndexByte(substr, ' ')
			var refname, message string
			if pos == -1 {
				refname = substr
			} else {
				refname = substr[0:pos]
				message = substr[pos+1:]
			}
			result = &Report{ReferenceName: plumbing.ReferenceName(refname), Reason: message, Rejected: true}
			break
		}
		if lab == "status" {
			// multiline-status
			for s := range strings.SplitSeq(line[pos+1:], "\n") {
				_, _ = term.SanitizedF("remote: %s\n", s)
			}
			continue
		}
	}
	if result == nil {
		if r.Err() != nil {
			return nil, r.Err()
		}
		return nil, io.ErrUnexpectedEOF
	}
	return
}
