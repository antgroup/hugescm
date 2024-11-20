package trace

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func Location(skip int) (string, int) {
	pc, _, line, ok := runtime.Caller(skip)
	if !ok {
		return "?", line
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "?", line
	}
	return fn.Name(), line
}

func Errorf(format string, a ...any) error {
	fn, line := Location(2)
	msg := fmt.Sprintf(format, a...)
	logrus.Error(fn, ":", line, " ", msg)
	return errors.New(msg)
}

type Tracker struct {
	debug bool
	last  time.Time
}

func NewTracker(debugMode bool) *Tracker {
	return &Tracker{debug: debugMode, last: time.Now()}
}

func (t *Tracker) StepNext(format string, a ...any) {
	if !t.debug {
		return
	}
	s := fmt.Sprintf(format, a...)
	now := time.Now()
	fmt.Fprintf(os.Stderr, "\x1b[35m* %s use time: %v\x1b[0m\n", strings.Trim(s, "\n"), now.Sub(t.last))
	t.last = now
}
