//go:build !windows

package strengthen

import (
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestDu(t *testing.T) {
	sz, err := Du("/tmp/repositories")
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable du %v\n", err)
		return
	}
	var si syscall.Stat_t
	if err := syscall.Stat("/tmp/repositories", &si); err != nil {
		fmt.Fprintf(os.Stderr, "unable du %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "/tmp/repositories %0.2f\n", float64(sz)/1024)
}
