package zeta

import (
	"bytes"
	"crypto/rand"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/term"
)

func TestProcessColor(t *testing.T) {
	b := make([]byte, 1000)
	_, err := rand.Read(b[10:])
	if err != nil {
		return
	}
	_ = processColor(bytes.NewReader(b), os.Stdout, int64(len(b)), term.Level16M)
}

func TestProcessColorOverflow(t *testing.T) {
	b := make([]byte, 1000)
	_, err := rand.Read(b[10:])
	if err != nil {
		return
	}
	_ = processColor(bytes.NewReader(b), os.Stdout, int64(len(b))+8, term.Level16M)
}

func TestBorder(t *testing.T) {
	input := make([]byte, 15)
	_, err := rand.Read(input)
	if err != nil {
		return
	}
	b := newBinaryPrinter(os.Stderr, term.Level16M)
	_ = b.writeBorder()
	_ = b.writeLine(0, input)
	_ = b.writeLine(16, []byte("world"))
	_ = b.writeFooter()
}
