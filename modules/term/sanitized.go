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
	// charIndex is a lookup table for quick character classification.
	// Index corresponds to ASCII code (0-255), values are CHAR_* constants.
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

// handleAnsiColorSequence parses an ANSI color sequence at the start of text.
// If the sequence is valid and allowColor is true, it's written to b.
// Returns the length of the sequence consumed, or 0 if invalid.
//
// Valid format: ESC [ [<n> [; <n>]*] m
//
// References:
//   - https://github.com/gitgitgadget/git/pull/1853
//   - https://public-inbox.org/git/Z4bqMYKRP7Gva5St@tapette.crustytoothpaste.net/T/#t
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

// SanitizeANSI sanitizes ANSI sequences in content for safe terminal output.
//
// Behavior:
//   - If allowColor is true: ANSI color sequences are preserved
//   - If allowColor is false: All ANSI sequences are removed
//   - Control characters (except tab and newline) are converted to caret notation (^G, etc.)
//
// This is useful for displaying untrusted or external output safely in a TUI.
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

// SanitizedF formats according to a format specifier, sanitizes the result,
// and writes it to stderr. Color sequences are preserved based on StderrLevel.
//
// This is a convenience function for safely printing formatted output to stderr
// in TUI applications, ensuring control characters are converted to caret notation.
func SanitizedF(format string, a ...any) (int, error) {
	content := fmt.Sprintf(format, a...)
	return os.Stderr.WriteString(SanitizeANSI(content, StderrLevel != LevelNone))
}
