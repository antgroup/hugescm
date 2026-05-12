package command

import (
	"errors"
	"os/exec"

	"github.com/antgroup/hugescm/modules/strengthen"
)

const (
	NoDir = ""
)

func FromError(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := errors.AsType[*exec.ExitError](err); ok {
		if len(e.Stderr) > 0 {
			return strengthen.ByteCat([]byte(e.Error()), []byte(". stderr: "), e.Stderr)
		}
		return e.Error()
	}
	return err.Error()
}

func FromErrorCode(err error) int {
	if err == nil {
		return 0
	}
	if e, ok := errors.AsType[*exec.ExitError](err); ok {
		return e.ExitCode()
	}
	return -1
}
