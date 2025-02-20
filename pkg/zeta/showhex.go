package zeta

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/antgroup/hugescm/modules/term"
)

const (
	CN byte = 0 /* null    */
	CS byte = 1 /* space   */
	CP byte = 2 /* print   */
	CC byte = 3 /* control */
	CH byte = 4 /* high    */
)

var (
	colorTable = []byte{
		CN, CC, CC, CC, CC, CC, CC, CC, CC, CC, CS, CS, CS, CS, CC, CC, CC, CC, CC, CC, CC, CC, CC, CC, CC, CC,
		CC, CC, CC, CC, CC, CC, CS, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP,
		CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP,
		CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP,
		CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CP, CC, CH, CH,
		CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH,
		CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH,
		CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH,
		CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH,
		CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH, CH,
	}
	displayTable = []byte{
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25,
		0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38,
		0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f, 0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b,
		0x4c, 0x4d, 0x4e, 0x4f, 0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e,
		0x5f, 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71,
		0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x7b, 0x7c, 0x7d, 0x7e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
		0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e, 0x2e,
	}
	color256Index = []string{"\x1b[90m", "\x1b[92m", "\x1b[96m", "\x1b[95m", "\x1b[93m"}
	color24Index  = []string{"\x1b[90m", "\x1b[38;2;67;233;123m", "\x1b[38;2;0;201;255m", "\x1b[38;2;255;0;255m", "\x1b[38;2;254;225;64m"}
)

type binaryPrinter struct {
	*bytes.Buffer
	w          io.Writer
	colorIndex []string
}

func newBinaryPrinter(w io.Writer, colorMode term.Level) *binaryPrinter {
	byteBuffer := &bytes.Buffer{}
	byteBuffer.Grow(400)
	colorTable := color256Index
	if colorMode == term.Level16M {
		colorTable = color24Index
	}
	return &binaryPrinter{Buffer: byteBuffer, w: w, colorIndex: colorTable}
}

// left_corner: '┌',
// horizontal_line: '─',
// column_separator: '┬',
// right_corner: '┐',
// left_corner: '└',
// horizontal_line: '─',
// column_separator: '┴',
// right_corner: '┘',
// │ ┊
const (
	// Hexadecimal => 2
	// [ 4d 5a 90 00 03 00 00 00 ]
	panelSize = 2*8 + 9
)

func (b *binaryPrinter) doPrintln(a ...string) {
	for _, s := range a {
		_, _ = b.WriteString(s)
	}
	_ = b.WriteByte('\n')
}
func (b *binaryPrinter) writeBorder() error {
	pannelStr := strings.Repeat("─", panelSize)
	h8 := strings.Repeat("─", 8)
	b.doPrintln("┌", h8, "┬", pannelStr, "┬", pannelStr, "┬", h8, "┬", h8, "┐")
	return b.flush()
}

func (b *binaryPrinter) writeFooter() error {
	pannelStr := strings.Repeat("─", panelSize)
	h8 := strings.Repeat("─", 8)
	b.doPrintln("└", h8, "┴", pannelStr, "┴", pannelStr, "┴", h8, "┴", h8, "┘")
	return b.flush()
}

func (b *binaryPrinter) formatByte(v byte) {
	c := colorTable[v]
	fmt.Fprintf(b.Buffer, "%s%02x\x1b[0m ", b.colorIndex[c], v)
}

func (b *binaryPrinter) displayByte(v byte) {
	c := colorTable[v]
	fmt.Fprintf(b.Buffer, "%s%c\x1b[0m", b.colorIndex[c], displayTable[v])
}

func (b *binaryPrinter) writeLine(offset int64, input []byte) error {
	fmt.Fprintf(b.Buffer, "│\x1b[90m%08x\x1b[0m│ ", offset)
	var i int
	for ; i < min(8, len(input)); i++ {
		b.formatByte(input[i])
	}
	for ; i < 8; i++ {
		_, _ = b.WriteString("   ")
	}
	_, _ = b.WriteString("┊ ")
	for ; i < min(16, len(input)); i++ {
		b.formatByte(input[i])
	}
	for ; i < 16; i++ {
		_, _ = b.WriteString("   ")
	}
	_, _ = b.WriteString("│")
	var j int
	for ; j < min(8, len(input)); j++ {
		b.displayByte(input[j])
	}
	for ; j < 8; j++ {
		_, _ = b.WriteString(" ")
	}
	_, _ = b.WriteString("┊")
	for ; j < min(16, len(input)); j++ {
		b.displayByte(input[j])
	}
	for ; j < 16; j++ {
		_, _ = b.WriteString(" ")
	}
	_, _ = b.WriteString("│\n")
	return b.flush()
}

func (b *binaryPrinter) flush() error {
	_, err := b.w.Write(b.Bytes())
	b.Reset()
	return err
}

func processColor(r io.Reader, w io.Writer, size int64, colorMode term.Level) error {
	if size < 0 {
		size = math.MaxInt64
	}
	var input [16]byte
	b := newBinaryPrinter(w, colorMode)
	if err := b.writeBorder(); err != nil {
		return err
	}
	var offset int64
	for {
		readBytes := min(size, 16)
		n, err := io.ReadFull(r, input[:readBytes])
		if err != nil && err != io.ErrUnexpectedEOF {
			break
		}
		if err := b.writeLine(offset, input[:n]); err != nil {
			return err
		}
		size -= int64(n)
		if size <= 0 {
			break
		}
		if n != 16 {
			break
		}
		offset += 16
	}
	return b.writeFooter()
}
