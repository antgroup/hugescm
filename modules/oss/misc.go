// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type RangeReader interface {
	io.Reader
	io.Closer
	Size() int64
	Range() string
}

type rangeReader struct {
	io.Reader
	closer io.Closer
	size   int64
	hdr    string
}

func (r *rangeReader) Close() error {
	if r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

func (r *rangeReader) Size() int64 {
	return r.size
}

func (r *rangeReader) Range() string {
	return r.hdr
}

func NewRangeReader(rc io.ReadCloser, size int64, hdr string) RangeReader {
	return &rangeReader{Reader: rc, closer: rc, size: size, hdr: hdr}
}

// https://developer.mozilla.org/zh-CN/docs/Web/HTTP/Headers/Content-Range
const (
	unitBytes = "bytes"
)

var (
	ErrNoSizeFromRange = errors.New("no size from range")
)

// Content-Range: <unit> <range-start>-<range-end>/<size>
// Content-Range: <unit> <range-start>-<range-end>/*
// Content-Range: <unit> */<size>
func parseSizeFromRange(hdr string) (int64, error) {
	pos := strings.IndexByte(hdr, ' ')
	if pos == -1 {
		return 0, ErrNoSizeFromRange
	}
	if hdr[:pos] != unitBytes {
		return 0, ErrNoSizeFromRange
	}
	sv := strings.FieldsFunc(hdr[pos+1:], func(r rune) bool {
		return r == '-' || r == '/'
	})
	if len(sv) == 2 {
		if sv[0] != "*" {
			return 0, ErrNoSizeFromRange
		}
		size, err := strconv.ParseInt(sv[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse size from range %s %v", hdr, err)
		}
		return size, nil
	}
	if len(sv) != 3 || sv[2] == "*" {
		return 0, ErrNoSizeFromRange
	}
	size, err := strconv.ParseInt(sv[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse size from range %s %v", hdr, err)
	}
	return size, nil
}
