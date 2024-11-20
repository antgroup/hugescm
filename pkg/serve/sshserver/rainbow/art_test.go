package rainbow

import (
	"os"
	"testing"
)

func TestWriteArt(t *testing.T) {
	Display(os.Stderr, &DisplayOpts{
		UserName:    "Jobs",
		KeyType:     "ssh-ed25519",
		Fingerprint: "SHA256:AagtCe13KpEjkDwnhKMplHHjhDG3m1YtwcfzLuPxey4",
		Width:       80,
	})
}
