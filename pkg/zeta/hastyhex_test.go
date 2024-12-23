package zeta

import (
	"bytes"
	"crypto/rand"
	"os"
	"testing"
)

func TestProcessColor(t *testing.T) {
	b := make([]byte, 1000)
	_, err := rand.Read(b[10:])
	if err != nil {
		return
	}
	_ = processColor(bytes.NewReader(b), os.Stdout, int64(len(b)))
}

func TestBorder(t *testing.T) {
	input := make([]byte, 15)
	_, err := rand.Read(input)
	if err != nil {
		return
	}
	b := newBinaryPrinter(os.Stderr)
	_ = b.writeBorder()
	_ = b.writeLine(0, input)
	_ = b.writeLine(16, []byte("world"))
	_ = b.writeFooter()
}
