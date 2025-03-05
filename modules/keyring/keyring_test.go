package keyring

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	service  = "test-service"
	user     = "test-user"
	password = "test-password"
)

// TestStore tests setting a user and password in the keyring.
func TestStore(t *testing.T) {
	err := Store(t.Context(), service, &Cred{UserName: user, Password: password})
	if err != nil {
		t.Errorf("Should not fail, got: %s", err)
	}
}

func TestEncodePassword(t *testing.T) {
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	encodedCred, _, err := transform.Bytes(encoder, []byte("My Password 你好 🦚"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "my password: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%x\n", encodedCred)
	dec := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
	password, _, err := transform.Bytes(dec, encodedCred)
	if err != nil {
		fmt.Fprintf(os.Stderr, "my password: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Password: %v\n", string(password))
}
