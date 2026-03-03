package term

import (
	"fmt"
	"io"
	"os"
	"regexp"
)

// ansiRegex is a regular expression that matches ANSI escape sequences.
const (
	ansiRegex = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
)

var (
	trimAnsiRegex = regexp.MustCompile(ansiRegex)
)

// StripANSI removes all ANSI escape sequences from the given string.
// This is useful for calculating string lengths or when displaying
// text in environments that don't support ANSI codes.
func StripANSI(s string) string {
	return trimAnsiRegex.ReplaceAllString(s, "")
}

// Fprintf formats according to a format specifier and writes to w.
// It respects the global StderrLevel and StdoutLevel settings:
//   - If w is os.Stdout and StdoutLevel is LevelNone, ANSI codes are stripped
//   - If w is os.Stderr and StderrLevel is LevelNone, ANSI codes are stripped
//   - Otherwise, output is passed through unchanged
//
// This allows TUI applications to automatically disable colors when
// the output is redirected to a file or pipe.
func Fprintf(w io.Writer, format string, a ...any) (int, error) {
	switch {
	case w == os.Stderr && StderrLevel == LevelNone:
		out := fmt.Sprintf(format, a...)
		return os.Stderr.WriteString(StripANSI(out))
	case w == os.Stdout && StdoutLevel == LevelNone:
		out := fmt.Sprintf(format, a...)
		return os.Stdout.WriteString(StripANSI(out))
	default:
	}
	return fmt.Fprintf(w, format, a...)
}
