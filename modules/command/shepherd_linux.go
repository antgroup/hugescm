//go:build linux

package command

import (
	"os/exec"
	"syscall"
)

func setSysProcAttribute(c *exec.Cmd, detached bool) {
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if !detached {
		c.SysProcAttr.Pdeathsig = syscall.SIGTERM
	}
}

func cleanExit(c *exec.Cmd, detached bool) {
	if c.Process == nil || c.Process.Pid <= 0 {
		return
	}
	if c.SysProcAttr != nil && c.SysProcAttr.Setpgid && !detached {
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
		return
	}
	_ = syscall.Kill(c.Process.Pid, syscall.SIGTERM)
}
