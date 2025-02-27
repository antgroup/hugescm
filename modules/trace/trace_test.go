package trace

import (
	"testing"

	"github.com/antgroup/hugescm/modules/term"
)

func TestDebug(t *testing.T) {
	term.StderrLevel = term.Level256
	d := NewDebugger(true)
	d.DbgPrint("jack")
}
