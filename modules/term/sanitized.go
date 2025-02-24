package term

import (
	"fmt"
	"os"
	"strings"
)

const (
	CHAR_UNSPECIFIED    = 0
	CHAR_COLOR_SEQUENCE = 1
	CHAR_CONTROL        = 2
)

var (
	charIndex = []byte{
		2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
)

// https://github.com/gitgitgadget/git/pull/1853
// https://public-inbox.org/git/Z4bqMYKRP7Gva5St@tapette.crustytoothpaste.net/T/#t
func handleAnsiColorSequence(b *strings.Builder, text []byte, allowColor bool) int {
	/*
	 * Valid ANSI color sequences are of the form
	 *
	 * ESC [ [<n> [; <n>]*] m
	 */
	if len(text) < 3 || text[0] != '\x1b' || text[1] != '[' {
		return 0
	}
	for i := 2; i < len(text); i++ {
		c := text[i]
		if c == 'm' {
			if allowColor {
				_, _ = b.Write(text[:i+1])
			}
			return i
		}
		if charIndex[c] != CHAR_COLOR_SEQUENCE {
			break
		}
	}
	return 0
}

func SanitizeANSI(content string, allowColor bool) string {
	b := &strings.Builder{}
	text := []byte(content)
	b.Grow(len(content))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if charIndex[c] != CHAR_CONTROL || c == '\t' || c == '\n' {
			_ = b.WriteByte(c)
			continue
		}
		if j := handleAnsiColorSequence(b, text[i:], allowColor); j != 0 {
			i += j
			continue
		}
		_ = b.WriteByte('^')
		_ = b.WriteByte(c + 0x40)
	}
	return b.String()
}

func SanitizedF(format string, a ...any) (int, error) {
	content := fmt.Sprintf(format, a...)
	return os.Stderr.WriteString(SanitizeANSI(content, StderrLevel != LevelNone))
}
