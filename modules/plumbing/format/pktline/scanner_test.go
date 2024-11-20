package pktline

import (
	"fmt"
	"os"
	"testing"
)

func TestHexDecode(t *testing.T) {
	ss := []string{
		"0014", "ffff", "abcd", "wwwww", "1186", "0000",
	}
	for _, s := range ss {
		var b [lenSize]byte
		copy(b[:], []byte(s))
		v, err := hexDecode(b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			continue
		}
		fmt.Fprintf(os.Stderr, "%s %d 0x%04x\n", s, v, v)
	}
}
