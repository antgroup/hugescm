package git

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestHash(t *testing.T) {
	h, err := Hash(context.Background(), ".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "hash: %v\n", h)
}

func TestParseReference(t *testing.T) {
	hash, refname, err := ParseReference(context.Background(), ".", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "RevParseEx error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Hash: [%s] Ref: [%s]\n", hash, refname)
}
