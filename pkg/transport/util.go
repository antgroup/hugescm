package transport

import (
	"bufio"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func NewObjectsReader(objects []plumbing.Hash) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		w := bufio.NewWriter(pw)
		for _, o := range objects {
			if _, err := w.WriteString(o.String()); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if err := w.WriteByte('\n'); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
		if err := w.WriteByte('\n'); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := w.Flush(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	return pr
}
