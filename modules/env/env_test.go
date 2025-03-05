package env

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnviron(t *testing.T) {
	for range 10 {
		now := time.Now()
		env := Environ()
		fmt.Fprintf(os.Stderr, "use time: %v\n", time.Since(now))
		fmt.Fprintf(os.Stderr, "%s\n", strings.Join(env, "\n"))
	}
}
