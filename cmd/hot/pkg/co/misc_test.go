package co

import (
	"fmt"
	"os"
	"testing"
)

func TestNewUserAgent(t *testing.T) {
	u, ok := NewUserAgent()
	if ok {
		fmt.Fprintf(os.Stderr, "New user-agent: %s\n", u)
	}
}
