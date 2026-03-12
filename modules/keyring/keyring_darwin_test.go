//go:build darwin

package keyring

import (
	"fmt"
	"os"
	"testing"
)

func TestGet(t *testing.T) {
	cred, err := Get(t.Context(), &Cred{Server: "http://zeta.io"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "find cred error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%v\n", cred)
}
