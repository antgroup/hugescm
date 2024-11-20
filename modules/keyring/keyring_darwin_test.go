//go:build darwin

package keyring

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestFind(t *testing.T) {
	o := &macOSXKeychain{}
	cred, err := o.Find(context.Background(), "http://zeta.io")
	if err != nil {
		fmt.Fprintf(os.Stderr, "find cred error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%v\n", cred)
}
