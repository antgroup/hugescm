package command

import (
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
	if e, ok := err.(*exec.ExitError); ok {
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
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	return -1
}
