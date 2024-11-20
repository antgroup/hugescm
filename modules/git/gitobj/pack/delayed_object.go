package pack

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strings"
)

// delayedObjectReader provides an interface for reading from an Object while
// loading object data into memory only on demand.  It implements io.ReadCloser.
type delayedObjectReader struct {
	obj     *Object
	mr      io.Reader
	closeFn func() error
}

func (d *delayedObjectReader) makeReader() (err error) {
	if b, ok := d.obj.data.(*ChainBase); ok {
		zr, err := zlib.NewReader(&OffsetReaderAt{
			r: b.r,
			o: b.offset,
		})
		if err != nil {
			return err
		}
		d.mr = io.MultiReader(
			// Git object header:
			strings.NewReader(fmt.Sprintf("%s %d\x00",
				b.typ.String(), b.size,
			)),

			// Git object (uncompressed) contents:
			io.LimitReader(zr, b.size),
		)
		d.closeFn = func() error {
			return zr.Close()
		}
		return nil
	}
	data, err := d.obj.Unpack()
	if err != nil {
		return err
	}
	d.mr = io.MultiReader(
		// Git object header:
		strings.NewReader(fmt.Sprintf("%s %d\x00",
			d.obj.Type(), len(data),
		)),

		// Git object (uncompressed) contents:
		bytes.NewReader(data),
	)
	return
}

// Read implements the io.Reader method by instantiating a new underlying reader
// only on demand.
func (d *delayedObjectReader) Read(b []byte) (int, error) {
	if d.mr == nil {
		if err := d.makeReader(); err != nil {
			return 0, err
		}
	}
	return d.mr.Read(b)
}

// Close implements the io.Closer interface.
func (d *delayedObjectReader) Close() error {
	if d.closeFn != nil {
		return d.closeFn()
	}
	return nil
}
