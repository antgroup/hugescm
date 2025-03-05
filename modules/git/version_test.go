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
		v, err := Version()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s use time: %v\n", v, time.Since(now))
	}
}
