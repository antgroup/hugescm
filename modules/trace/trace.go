package trace

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/term"
)

var (
	verbose bool
)

func EnableDebugMode() {
	verbose = true
}

func DbgPrint(format string, args ...any) {
	if !verbose {
		return
	}
	message := fmt.Sprintf(format, args...)
	var buffer bytes.Buffer
	switch term.StderrLevel {
	case term.Level16M:
		for s := range strings.SplitSeq(message, "\n") {
			_, _ = buffer.WriteString("\x1b[38;2;254;225;64m* ")
			_, _ = buffer.WriteString(s)
			_, _ = buffer.WriteString("\x1b[0m\n")
		}
	case term.Level256:
		for s := range strings.SplitSeq(message, "\n") {
			_, _ = buffer.WriteString("\x1b[33m* ")
			_, _ = buffer.WriteString(s)
			_, _ = buffer.WriteString("\x1b[0m\n")
		}
	default:
		for s := range strings.SplitSeq(message, "\n") {
			_, _ = buffer.WriteString(s)
			_ = buffer.WriteByte('\n')
		}
	}
	_, _ = os.Stderr.Write(buffer.Bytes())
}
