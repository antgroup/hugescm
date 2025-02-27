package strengthen

import (
	"fmt"
	"os"
	"testing"
)

func TestNewRID(t *testing.T) {
	fmt.Fprintf(os.Stderr, "new uuid: {%s}\n", NewRID())
	fmt.Fprintf(os.Stderr, "new fingerprint: {%s}\n", NewRandomString(16))
}

func TestNewToken(t *testing.T) {
	fmt.Fprintf(os.Stderr, "new token: {%s}\n", NewToken())
}
