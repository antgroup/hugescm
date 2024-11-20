package oss

import (
	"fmt"
	"os"
	"testing"
)

func TestSizeFromRange(t *testing.T) {
	ss := []string{
		"bytes 200-1000/67589",
		"bytes 100-900/344606",
		"bytes 100-900/*",
		"bytes */344606",
		"x",
	}
	for _, s := range ss {
		i, err := parseSizeFromRange(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hdr: %s error: %v\n", s, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "hdr: %s size %d \n", s, i)
	}
}
