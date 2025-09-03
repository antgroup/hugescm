package git

import (
	"io"
)

// object metadata
type Metadata struct {
	// Hash of the object.
	Hash string
	// Size is the total uncompressed size of the blob's contents.
	Size int64
	// Type of the object
	Type ObjectType
}

type Object struct {
	// Hash of the object.
	Hash string
	// Size is the total uncompressed size of the blob's contents.
	Size int64
	// Type of the object
	Type ObjectType
	// dataReader is a reader that yields the uncompressed blob contents. It
	// may only be read once.
	dataReader io.Reader
}

func (o *Object) Read(p []byte) (int, error) {
	return o.dataReader.Read(p)
}

// WriteTo implements the io.WriterTo interface. It defers the write to the embedded object reader
// via `io.Copy()`, which in turn will use `WriteTo()` or `ReadFrom()` in case these interfaces are
// implemented by the respective reader or writer.
func (o *Object) WriteTo(w io.Writer) (int64, error) {
	// `io.Copy()` will make use of `ReadFrom()` in case the writer implements it.
	return io.Copy(w, o.dataReader)
}

func (o *Object) Discard() {
	if o.dataReader != nil {
		_, _ = io.Copy(io.Discard, o.dataReader)
	}
}

type Printer interface {
	Pretty(io.Writer) error
}
