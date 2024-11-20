package command

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewCommand(t *testing.T) {
	cmd := New(context.Background(), ".", "git", "version")
	line, err := cmd.OneLine()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nCount: %d\n", line, ProcessesCount())
}

func TestNewCommand2(t *testing.T) {
	var stdout strings.Builder
	cmd := NewFromOptions(context.Background(), &RunOpts{RepoPath: ".", Stdout: &stdout}, "git", "version")
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[%s]\nCount: %d\n", stdout.String(), ProcessesCount())
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[%s]\nCount: %d\n", stdout.String(), ProcessesCount())
}

func TestNewCommand3(t *testing.T) {
	cmd := New(context.Background(), ".", "git", "version---")
	b, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\nCount: %d\n", FromError(err), ProcessesCount())
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nCount: %d\n", b, ProcessesCount())
}

func TestNewCommand4(t *testing.T) {
	cmd := New(context.Background(), ".", "git", "help")
	b, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\nCount: %d\n", FromError(err), ProcessesCount())
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nCount: %d\nuse time: %v\n", b, ProcessesCount(), cmd.UseTime())
}

func TestWaitTimeout(t *testing.T) {
	newCtx, cancelCtx := context.WithTimeout(context.Background(), time.Second*4)
	defer cancelCtx()
	cmd := NewFromOptions(newCtx, &RunOpts{
		Stderr: os.Stderr,
		Stdout: os.Stdout,
		Stdin:  os.Stdin,
	}, "git", "upload-pack", "/tmp/ssh.git")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\nCount: %d\n", FromError(err), ProcessesCount())
		return
	}
}

func TestChildProcess(t *testing.T) {
	newCtx, cancelCtx := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelCtx()
	cmd := NewFromOptions(newCtx, &RunOpts{
		Stderr: os.Stderr,
		Stdout: os.Stdout,
		Stdin:  os.Stdin,
	}, "sh", "-c", "git upload-pack /root/dev/batman/.git")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\nCount: %d\n", FromError(err), ProcessesCount())
		return
	}
}
