package streamio

import (
	"bufio"
	"io"
	"sync"
)

var bufioReader = sync.Pool{
	New: func() any {
		return bufio.NewReader(nil)
	},
}

// GetBufioReader returns a *bufio.Reader that is managed by a sync.Pool.
// Returns a bufio.Reader that is reset with reader and ready for use.
//
// After use, the *bufio.Reader should be put back into the sync.Pool
// by calling PutBufioReader.
func GetBufioReader(reader io.Reader) *bufio.Reader {
	r := bufioReader.Get().(*bufio.Reader)
	r.Reset(reader)
	return r
}

// PutBufioReader puts reader back into its sync.Pool.
func PutBufioReader(reader *bufio.Reader) {
	bufioReader.Put(reader)
}

const (
	largePacketSize = 64 * 1024
)

var bufferWriter = sync.Pool{
	New: func() any {
		return bufio.NewWriterSize(nil, largePacketSize)
	},
}

// GetBufferWriter returns a *bufio.Writer that is managed by a sync.Pool.
// Returns a bufio.Writer that is reset with writer and ready for use.
//
// After use, the *bufio.Writer should be put back into the sync.Pool
// by calling PutBufferWriter.
func GetBufferWriter(writer io.Writer) *bufio.Writer {
	w := bufferWriter.Get().(*bufio.Writer)
	w.Reset(writer)
	return w
}

// PutBufferWriter puts reader back into its sync.Pool.
func PutBufferWriter(writer *bufio.Writer) {
	bufferWriter.Put(writer)
}

func LargeCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	w := GetBufferWriter(dst)
	defer PutBufferWriter(w)
	if written, err = io.Copy(w, src); err != nil {
		return
	}
	err = w.Flush()
	return
}
