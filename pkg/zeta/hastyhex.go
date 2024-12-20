package zeta

import (
	"io"
	"math"
)

const (
	CN            byte = 0x37 /* null    */
	CS            byte = 0x92 /* space   */
	CP            byte = 0x96 /* print   */
	CC            byte = 0x95 /* control */
	CH            byte = 0x93 /* high    */
	colorTemplate      = "00000000  " +
		"\x1b[XXm## \x1b[XXm## \x1b[XXm## \x1b[XXm## " +
		"\x1b[XXm## \x1b[XXm## \x1b[XXm## \x1b[XXm##  " +
		"\x1b[XXm## \x1b[XXm## \x1b[XXm## \x1b[XXm## " +
		"\x1b[XXm## \x1b[XXm## \x1b[XXm## \x1b[XXm##  " +
		"\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm." +
		"\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm.\x1b[XXm." +
		"\x1b[0m\n"
)

var (
	hex   = []byte("0123456789abcdef")
	table = []byte{
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
	slots = []int{ /* ANSI-color, hex, ANSI-color, ASCII */
		12, 15, 142, 145, 20, 23, 148, 151, 28, 31, 154, 157, 36, 39, 160, 163,
		44, 47, 166, 169, 52, 55, 172, 175, 60, 63, 178, 181, 68, 71, 184, 187,
		77, 80, 190, 193, 85, 88, 196, 199, 93, 96, 202, 205, 101, 104, 208, 211,
		109, 112, 214, 217, 117, 120, 220, 223, 125, 128, 226, 229, 133, 136, 232, 235}
)

func processColor(r io.Reader, w io.Writer, size int64) error {
	var input [16]byte
	colortemplate := []byte(colorTemplate)
	if size < 0 {
		size = math.MaxInt64
	}
	var offset int64
	for {
		rn := min(size, 16)
		n, err := io.ReadFull(r, input[:rn])
		if err != nil {
			break
		}
		/* Write the offset */
		for i := 0; i < 8; i++ {
			colortemplate[i] = hex[(offset>>(28-i*4))&15]
		}
		size -= int64(n)
		/* Fill out the colortemplate */
		for i := 0; i < 16; i++ {
			/* Use a fixed loop count instead of "n" to encourage loop
			 * unrolling by the compiler. Empty bytes will be erased
			 * later.
			 */
			v := input[i]
			c := table[v]
			colortemplate[slots[i*4+0]+0] = hex[c>>4]
			colortemplate[slots[i*4+0]+1] = hex[c&15]
			colortemplate[slots[i*4+1]+0] = hex[v>>4]
			colortemplate[slots[i*4+1]+1] = hex[v&15]
			colortemplate[slots[i*4+2]+0] = hex[c>>4]
			colortemplate[slots[i*4+2]+1] = hex[c&15]
			colortemplate[slots[i*4+3]+0] = displayTable[v]
		}
		/* Erase any trailing bytes */
		for i := n; i < 16; i++ {
			/* This loop is only used once: the last line of output. The
			 * branch predictor will quickly learn that it's never taken.
			 */
			colortemplate[slots[i*4+0]+0] = '0'
			colortemplate[slots[i*4+0]+1] = '0'
			colortemplate[slots[i*4+1]+0] = ' '
			colortemplate[slots[i*4+1]+1] = ' '
			colortemplate[slots[i*4+2]+0] = '0'
			colortemplate[slots[i*4+2]+1] = '0'
			colortemplate[slots[i*4+3]+0] = ' '
		}
		if _, err := w.Write(colortemplate); err != nil {
			return err
		}
		offset += 16
		if n != 16 || size <= 0 {
			break
		}
	}
	return nil
}
