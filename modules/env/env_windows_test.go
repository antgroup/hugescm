//go:build windows

package env

import (
	"fmt"
	"os"
	"testing"
)

func TestInitializeEnv(t *testing.T) {
	_ = os.Setenv("PATH", os.Getenv("PATH")+";C:\\Windows")
	if err := DelayInitializeEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "initialize env error: %v\n", err)
	}
}

func TestLookupPager(t *testing.T) {
	lessExe, err := LookupPager("less")
	if err != nil {
		fmt.Fprintf(os.Stderr, "search less exe error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "found less: %v\n", lessExe)
}
