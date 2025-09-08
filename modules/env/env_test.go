package env

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnviron(t *testing.T) {
	now := time.Now()
	env := Environ()
	fmt.Fprintf(os.Stderr, "use time: %v\n", time.Since(now))
	fmt.Fprintf(os.Stderr, "%s\n", strings.Join(env, "\n"))
}

func TestEnvironForEach(t *testing.T) {
	for range 10 {
		now := time.Now()
		env := Environ()
		fmt.Fprintf(os.Stderr, "%d use time: %v\n", len(env), time.Since(now))
	}
}

func TestSanitizeEnv(t *testing.T) {
	for _, e := range SanitizeEnv("PATH") {
		fmt.Fprintf(os.Stderr, "%s\n", e)
	}
}
