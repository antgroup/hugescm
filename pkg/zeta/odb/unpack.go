// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/antgroup/hugescm/modules/crc"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/progress"
	"github.com/antgroup/hugescm/pkg/tr"
)

var (
	metadataStreamMagic = [4]byte{'Z', 'M', '\x00', '\x01'}
	blobStreamMagic     = [4]byte{'Z', 'B', '\x00', '\x02'}
)

func (d *ODB) MetadataUnpack(r io.Reader, quiet bool) error {
	start := time.Now()
	ur, err := d.NewUnpacker(0, true)
	if err != nil {
		return err
	}
	defer ur.Close()
	b := progress.NewUnknownBar(tr.W("Metadata downloading"), quiet)
	cr := crc.NewCrc64Reader(b.NewTeeReader(r))
	var magic, version [4]byte
	var reserved [16]byte
	if _, err := io.ReadFull(cr, magic[:]); err != nil {
		b.Exit()
		return err
	}
	if !bytes.Equal(magic[:], metadataStreamMagic[:]) {
		b.Exit()
		err = fmt.Errorf("unexpected metadata '%c' '%c' '%c' '%c'", magic[0], magic[1], magic[2], magic[3])
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	if _, err := io.ReadFull(cr, version[:]); err != nil {
		b.Exit()
		fmt.Fprintf(os.Stderr, "unexpected metadata version error: %v\n", err)
		return err
	}
	if _, err := io.ReadFull(cr, reserved[:]); err != nil {
		b.Exit()
		fmt.Fprintf(os.Stderr, "unexpected reserved, error: %v\n", err)
		return err
	}
	var oidBytes [64]byte
	var count int
	var readBytes int64
	for {
		var length uint32
		if err := binary.Read(cr, binary.BigEndian, &length); err != nil {
			b.Exit()
			fmt.Fprintf(os.Stderr, "unexpected metadata length, error: %v\n", err)
			return err
		}
		if length == 0 {
			break
		}
		count++
		if _, err = io.ReadFull(cr, oidBytes[:]); err != nil {
			b.Exit()
			err := fmt.Errorf("unexpected metadata hash, err: %v", err)
			fmt.Fprint(os.Stderr, err)
			return err
		}
		objectSize := length - plumbing.HASH_HEX_SIZE
		readBytes += int64(objectSize)
		if err := ur.Write(plumbing.NewHash(string(oidBytes[:])), objectSize, io.LimitReader(cr, int64(objectSize)), 0); err != nil {
			b.Exit()
			return err
		}
	}
	b.Finish()
	if err := cr.Verify(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := ur.Preserve(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s: %d <%s>, %s: %v\n", tr.W("Metadata download completed, total"), count, strengthen.FormatSize(readBytes), tr.W("time spent"), time.Since(start).Truncate(time.Millisecond))
	return nil
}

func (d *ODB) Unpack(r io.Reader, expected int, quiet bool) error {
	start := time.Now()
	ur, err := d.NewUnpacker(0, false)
	if err != nil {
		return err
	}
	defer ur.Close()
	cr := crc.NewCrc64Reader(r)
	var magic, version [4]byte
	var reserved [16]byte
	if _, err := io.ReadFull(cr, magic[:]); err != nil {
		fmt.Fprintf(os.Stderr, "fail to read blob transport magic, err: %v\n", err)
		return err
	}
	if !bytes.Equal(magic[:], blobStreamMagic[:]) {
		message := fmt.Sprintf("unexpected batch objects magic '%c' '%c' '%c' '%c'", magic[0], magic[1], magic[2], magic[3])
		fmt.Fprintln(os.Stderr, message)
		return errors.New(message)
	}
	if _, err := io.ReadFull(cr, version[:]); err != nil {
		fmt.Fprintf(os.Stderr, "unexpected batch objects version error: %v\n", err)
		return err
	}
	if _, err := io.ReadFull(cr, reserved[:]); err != nil {
		fmt.Fprintf(os.Stderr, "unexpected reserved, error: %v\n", err)
		return err
	}

	var oidBytes [64]byte
	var count int
	var readBytes int64
	b := progress.NewBar(tr.W("Batch download files"), expected, quiet)
	for {
		var length uint32
		if err := binary.Read(cr, binary.BigEndian, &length); err != nil {
			b.Exit()
			fmt.Fprintf(os.Stderr, "unexpected object length, error: %v\n", err)
			return err
		}
		if length == 0 {
			break
		}
		count++
		if _, err := io.ReadFull(cr, oidBytes[:]); err != nil {
			b.Exit()
			fmt.Fprintf(os.Stderr, "unexpected object hash, error: %v\n", err)
			return err
		}
		objectSize := length - plumbing.HASH_HEX_SIZE
		readBytes += int64(objectSize)
		if err := ur.Write(plumbing.NewHash(string(oidBytes[:])), objectSize, io.LimitReader(cr, int64(objectSize)), 0); err != nil {
			b.Exit()
			return err
		}
		b.Add(1)
	}
	if err := cr.Verify(); err != nil {
		b.Exit()
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	if err := ur.Preserve(); err != nil {
		b.Exit()
		return err
	}
	b.Finish()
	fmt.Fprintf(os.Stderr, "%s: %d <%s>, %s: %v\n", tr.W("Files download completed, total"), count, strengthen.FormatSize(readBytes), tr.W("time spent"), time.Since(start).Truncate(time.Millisecond))
	return nil
}
