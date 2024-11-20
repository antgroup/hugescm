package backend

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestPackObjects(t *testing.T) {
	opts := &PackOptions{
		ZetaDir: "/tmp/xh3/.zeta",
	}
	if err := PackObjects(context.Background(), opts); err != nil {
		fmt.Fprintf(os.Stderr, "pack objects error: %v\n", err)
	}
}
