package trace

import (
	"testing"

	"github.com/antgroup/hugescm/modules/term"
)

func TestDebug(t *testing.T) {
	term.StderrMode = term.HAS_256COLOR
	d := NewDebuger(true)
	d.DbgPrint("jack")
}
