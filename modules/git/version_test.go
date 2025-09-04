package git

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	for range 10 {
		now := time.Now()
		v, err := VersionDetect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s use time: %v\n", v, time.Since(now))
	}
}

func TestIsGitVersionAtLeast(t *testing.T) {
	fmt.Fprintf(os.Stderr, ">= 2.36.0: %v\n", IsGitVersionAtLeast(NewVersion(2, 36, 0)))
}
