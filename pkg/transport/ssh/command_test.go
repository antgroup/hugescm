package ssh

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/shlex"
	"github.com/antgroup/hugescm/modules/zeta"
)

func TestEscapeArgs(t *testing.T) {
	vv, err := shlex.Split("zeta-serve ls-remote '--reference=refs/heads/jack' 'repo/jack~1'", true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "EEE %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "args: %v\n", strings.Join(vv, ","))
}

type LastErrorCode struct {
	lastError *zeta.ErrExitCode
	err       error
}

func (c *LastErrorCode) LastErrorBAD() error {
	return c.lastError
}

func (c *LastErrorCode) LastErrorOK() error {
	if c.lastError == nil {
		return nil
	}
	return c.lastError
}

func (c *LastErrorCode) LastError2() error {
	return c.err
}

func TestLastError(t *testing.T) {
	var cc LastErrorCode
	badErr := cc.LastErrorBAD()
	badErr2 := badErr
	goodErr := cc.LastErrorOK()
	err2 := cc.LastError2()
	fmt.Fprintf(os.Stderr, "%v LastErrorBAD() is nil %v\n", badErr, badErr == nil)
	fmt.Fprintf(os.Stderr, "%v LastErrorBAD() is nil %v\n", badErr2, badErr2 == nil)
	fmt.Fprintf(os.Stderr, "%v LastErrorOK() is nil %v\n", goodErr, goodErr == nil)
	fmt.Fprintf(os.Stderr, "%v LastError2() is nil %v\n", err2, err2 == nil)
}
