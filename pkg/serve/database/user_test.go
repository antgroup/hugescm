package database

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestUser(t *testing.T) {
	fmt.Fprintf(os.Stderr, "%s\n", time.Time{})
	x, err := time.Parse(time.DateTime, "0000-00-00 00:00:00")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "0000-00-00 00:00:00 --> %v\n", x)
}

func TestUser2(t *testing.T) {
	fmt.Fprintf(os.Stderr, "%s\n", zeroLockedAt)
}
