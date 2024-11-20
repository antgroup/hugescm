package strengthen

import (
	"io"
)

type LimitWriter struct {
	dst   io.Writer
	limit int
}

// NewLimitWriter create a new LimitWriter that accepts at most 'limit' bytes.
func NewLimitWriter(dst io.Writer, limit int) *LimitWriter {
	return &LimitWriter{
		dst:   dst,
		limit: limit,
	}
}

func (w *LimitWriter) Write(p []byte) (int, error) {
	n := len(p)
	var err error
	if w.limit > 0 {
		if n > w.limit {
			p = p[:w.limit]
		}
		w.limit -= len(p)
		_, err = w.dst.Write(p)
	}
	return n, err
}
