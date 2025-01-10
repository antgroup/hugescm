package term

import (
	"fmt"
	"io"
	"os"
	"regexp"
)

const (
	ansiRegex = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
)

var (
	trimAnsiRegex = regexp.MustCompile(ansiRegex)
)

func StripANSI(s string) string {
	return trimAnsiRegex.ReplaceAllString(s, "")
}

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
