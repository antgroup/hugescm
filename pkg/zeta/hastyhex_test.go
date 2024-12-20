package zeta

import (
	"bytes"
	"crypto/rand"
	"os"
	"testing"
)

func TestProcessColor(t *testing.T) {
	b := make([]byte, 1000)
	_, err := rand.Read(b)
	if err != nil {
		return
	}
	processColor(bytes.NewReader(b), os.Stdout, int64(len(b)))

}
