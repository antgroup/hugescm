package term

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/ansi"
)

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
		return os.Stderr.WriteString(ansi.Strip(out))
	case w == os.Stdout && StdoutLevel == LevelNone:
		out := fmt.Sprintf(format, a...)
		return os.Stdout.WriteString(ansi.Strip(out))
	default:
	}
	return fmt.Fprintf(w, format, a...)
}
