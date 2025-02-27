package sshserver

import (
	"fmt"
	"os"
	"testing"
)

func TestNoCommand(t *testing.T) {
	args := []string{"jack", "ls"}
	if _, err := NewCommand(args); err != nil {
		fmt.Fprintf(os.Stderr, "parse command: %v\n", err)
	}
}

func TestLsRemoteCommand(t *testing.T) {
	args := []string{"ls-remote", "mono/zeta", "--reference", "refs/heads/mainline"}
	cmd, err := NewCommand(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse command: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%v\n", cmd)
}

func TestLsRemoteCommand2(t *testing.T) {
	args := []string{"ls-remote", "mono/zeta", "--reference=refs/heads/mainline"}
	cmd, err := NewCommand(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse command: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%v\n", cmd)
}

func TestMetadataCommand(t *testing.T) {
	args := []string{"metadata", "ls"}
	if _, err := NewCommand(args); err != nil {
		fmt.Fprintf(os.Stderr, "parse command: %v\n", err)
	}
}
