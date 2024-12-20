package streamio

import (
	"bytes"
	"io"
	"sync"
)

var (
	byteSlice = sync.Pool{
		New: func() any {
			b := make([]byte, 32*1024)
			return &b
		},
	}
	bytesBuffer = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(nil)
		},
	}
)

// GetByteSlice returns a *[]byte that is managed by a sync.Pool.
// The initial slice length will be 16384 (16kb).
//
// After use, the *[]byte should be put back into the sync.Pool
// by calling PutByteSlice.
func GetByteSlice() *[]byte {
	buf := byteSlice.Get().(*[]byte)
	return buf
}

// PutByteSlice puts buf back into its sync.Pool.
func PutByteSlice(buf *[]byte) {
	byteSlice.Put(buf)
}

// GetBytesBuffer returns a *bytes.Buffer that is managed by a sync.Pool.
// Returns a buffer that is reset and ready for use.
//
// After use, the *bytes.Buffer should be put back into the sync.Pool
// by calling PutBytesBuffer.
func GetBytesBuffer() *bytes.Buffer {
	buf := bytesBuffer.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutBytesBuffer puts buf back into its sync.Pool.
func PutBytesBuffer(buf *bytes.Buffer) {
	bytesBuffer.Put(buf)
}

// Copy copy reader to writer
func Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := GetByteSlice()
	defer PutByteSlice(buf)
	return io.CopyBuffer(dst, src, *buf)
}
