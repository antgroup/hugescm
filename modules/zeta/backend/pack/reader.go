// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package pack

import "io"

// SizeReader transforms an io.ReaderAt into an io.Reader by beginning and
// advancing all reads at the given offset.
type SizeReader struct {
	// raw is the data source for this instance of *OffsetReaderAt.
	raw io.ReaderAt

	// offset if the number of bytes read from the underlying data source, "r".
	// It is incremented upon reads.
	offset int64

	n    int64 // max bytes remaining
	size int64
}

func NewSizeReader(r io.ReaderAt, offset int64, size int64) *SizeReader {
	return &SizeReader{raw: r, offset: offset, n: size, size: size}
}

func (r *SizeReader) Size() int64 {
	return r.size
}

// close
func (r *SizeReader) Close() error {
	return nil
}

// Read implements io.Reader.Read by reading into the given []byte, "p" from the
// last known offset provided to the OffsetReaderAt.
//
// It returns any error encountered from the underlying data stream, and
// advances the reader forward by "n", the number of bytes read from the
// underlying data stream.
func (r *SizeReader) Read(p []byte) (n int, err error) {
	if r.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.n {
		p = p[0:r.n]
	}
	n, err = r.raw.ReadAt(p, r.offset)
	r.offset += int64(n)
	r.n -= int64(n)
	return
}
