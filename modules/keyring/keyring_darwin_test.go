//go:build darwin

package keyring

import (
	"testing"
)

func TestGet(t *testing.T) {
	cred, err := Get(t.Context(), &Cred{Server: "zeta.io"})
	if err != nil {
		if err == ErrNotFound {
			t.Skip("no credential found for zeta.io")
		}
		t.Fatalf("Get failed: %v", err)
	}
	t.Logf("found credential: username=%q, server=%q", cred.UserName, cred.Server)
}
