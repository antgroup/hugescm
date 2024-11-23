//go:build windows

package env

import (
	"fmt"
	"os"
	"testing"
)

func TestInitializeEnv(t *testing.T) {
	os.Setenv("PATH", os.Getenv("PATH")+";C:\\Windows")
	if err := InitializeEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "initialize env error: %v\n", err)
	}
}
