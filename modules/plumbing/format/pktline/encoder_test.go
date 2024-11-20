package pktline

import (
	"fmt"
	"os"
	"testing"
)

func TestEncodeLen(t *testing.T) {
	nums := []int{0, 1, 2, 3, 4, 7, 65535, 1000, 2000, 445, 7236}
	for _, n := range nums {
		fmt.Fprintf(os.Stderr, "%d %s %04x\n", n, asciiHex16(n), n)
	}
}
