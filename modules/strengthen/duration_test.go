package strengthen

import (
	"fmt"
	"os"
	"testing"
)

func TestDurationByte(t *testing.T) {
	for i := range 256 {
		if validDurationByte[i] == 1 {
			fmt.Fprintf(os.Stderr, "GOOD: %c\n", i)
		}
	}
}

func TestParseDuration(t *testing.T) {
	ss := []string{
		"-1.5h", "300ms", "2h45m", "uuuu8h",
	}
	for _, s := range ss {
		d, err := ParseDuration(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "BAD: %s err: %v\n", s, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "GOOD: %v\n", d)
	}
}
