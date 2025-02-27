package term

import (
	"fmt"
	"os"
	"testing"
	"unicode"
)

func TestStripAnsi(t *testing.T) {
	ss := fmt.Sprintf("\x1b[38;2;254;225;64m* %s jack\x1b[0m", os.Args[0])
	as := StripANSI(ss)
	fmt.Fprintf(os.Stderr, "%s\n", as)
}

func TestCygwinTerminal(t *testing.T) {
	fmt.Fprintf(os.Stderr, "IsCygwinTerminal: %v\n", IsCygwinTerminal(os.Stderr.Fd()))
}

func TestSanitized(t *testing.T) {
	ss := []string{
		"error: Have you \033[31mread\033[m this?\a\n",
		fmt.Sprintf("\x1b[38;2;254;225;64m* %s jack\x1b[0m", os.Args[0]),
	}
	for i, s := range ss {
		s1 := SanitizeANSI(s, true)
		s2 := SanitizeANSI(s, false)
		fmt.Fprintf(os.Stderr, "round %d\n%s\x1b[0m\n%s\x1b[0m\n", i, s1, s2)
	}
}

func TestTable(t *testing.T) {
	table := make([]int, 0, 256)
	for i := range 256 {
		// iscntrl: i < 0x20 || i == 0x7f
		if i < 0x20 || i == 0x7f {
			table = append(table, CHAR_CONTROL)
			continue
		}
		if unicode.IsDigit(rune(i)) || i == ';' || i == ':' {
			table = append(table, CHAR_COLOR_SEQUENCE)
			continue
		}
		table = append(table, CHAR_UNSPECIFIED)
	}
	for i, b := range table {
		if i%16 == 0 && i != 0 {
			fmt.Fprintf(os.Stderr, "\n")
		}
		fmt.Fprintf(os.Stderr, "%d,", b)
	}
}

func TestSanitizedF(t *testing.T) {
	_, _ = SanitizedF("remote: %s\n", "objects 已验证")
	_, _ = SanitizedF("remote: %s\n", "objects 你好")
}
