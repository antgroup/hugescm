package trace

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/term"
)

type Debuger interface {
	DbgPrint(format string, args ...any)
}

func NewDebuger(verbose bool) Debuger {
	return &debuger{verbose: verbose}
}

type debuger struct {
	verbose bool
}

func DbgPrint(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	var buffer bytes.Buffer
	switch term.StderrLevel {
	case term.Level16M:
		for _, s := range strings.Split(message, "\n") {
			_, _ = buffer.WriteString("\x1b[38;2;254;225;64m* ")
			_, _ = buffer.WriteString(s)
			_, _ = buffer.WriteString("\x1b[0m\n")
		}
	case term.Level256:
		for _, s := range strings.Split(message, "\n") {
			_, _ = buffer.WriteString("\x1b[33m* ")
			_, _ = buffer.WriteString(s)
			_, _ = buffer.WriteString("\x1b[0m\n")
		}
	default:
		for _, s := range strings.Split(message, "\n") {
			_, _ = buffer.WriteString(s)
			_ = buffer.WriteByte('\n')
		}
	}
	_, _ = os.Stderr.Write(buffer.Bytes())
}

func (d debuger) DbgPrint(format string, args ...any) {
	if !d.verbose {
		return
	}
	DbgPrint(format, args...)
}

var (
	_ Debuger = &debuger{}
)
