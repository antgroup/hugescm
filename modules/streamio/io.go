package streamio

import (
	"bytes"
	"io"
)

func ReadMax(r io.Reader, n int64) ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(int(n))
	if _, err := buf.ReadFrom(io.LimitReader(r, n)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GrowReadMax(r io.Reader, n int64, grow int) ([]byte, error) {
	var buf bytes.Buffer
	if grow <= 0 {
		grow = int(n)
	}
	buf.Grow(grow)
	if _, err := buf.ReadFrom(io.LimitReader(r, n)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
