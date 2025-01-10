package progress

import (
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/term"
)

func TestNewBar(t *testing.T) {
	term.StderrMode = term.HAS_TRUECOLOR
	b := NewBar("init", 100, false)
	for i := 0; i < 100; i++ {
		time.Sleep(time.Millisecond * 100)
		b.Add(1)
	}
	b.Finish()
}
