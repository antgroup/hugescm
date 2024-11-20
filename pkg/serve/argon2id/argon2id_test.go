package argon2id

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestGenHash(t *testing.T) {
	now := time.Now()
	h, err := CreateHash("123456", DefaultParams)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s %v\n", h, time.Since(now))
}
