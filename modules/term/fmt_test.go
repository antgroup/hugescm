package term

import (
	"fmt"
	"os"
	"testing"
)

func TestStripAnsi(t *testing.T) {
	ss := fmt.Sprintf("\x1b[38;2;254;225;64m* %s jack\x1b[0m", os.Args[0])
	as := StripANSI(ss)
	fmt.Fprintf(os.Stderr, "%s\n", as)
}
