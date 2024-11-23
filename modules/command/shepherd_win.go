//go:build windows

package command

import "os/exec"

func setSysProcAttribute(c *exec.Cmd, detached bool) {
	// placeholders
}

func cleanExit(c *exec.Cmd, _ bool) {
	if c != nil && c.Process != nil {
		_ = c.Process.Kill()
	}
}
