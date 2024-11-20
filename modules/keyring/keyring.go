package keyring

import (
	"context"
	"errors"
)

// provider set in the init function by the relevant os file e.g.:
// keyring_unix.go
var provider Keyring = fallbackServiceProvider{}

var (
	// ErrNotFound is the expected error if the secret isn't found in the
	// keyring.
	ErrNotFound = errors.New("secret not found in keyring")
	// ErrSetDataTooBig is returned if `Set` was called with too much data.
	// On MacOS: The combination of service, username & password should not exceed ~3000 bytes
	// On Windows: The service is limited to 32KiB while the password is limited to 2560 bytes
	// On Linux/Unix: There is no theoretical limit but performance suffers with big values (>100KiB)
	ErrSetDataTooBig = errors.New("data passed to Set was too big")
)

type Cred struct {
	UserName string
	Password string
}

// Keyring provides a simple set/get interface for a keyring service.
type Keyring interface {
	// Find cred in keyring for target.
	Find(ctx context.Context, targetName string) (*Cred, error)
	// Store target cred
	Store(ctx context.Context, targetName string, c *Cred) error
	// Discard cred
	Discard(ctx context.Context, targetName string) error
}

// Find cred in keyring for target.
func Find(ctx context.Context, targetName string) (*Cred, error) {
	return provider.Find(ctx, targetName)
}

// Store target cred
func Store(ctx context.Context, targetName string, c *Cred) error {
	return provider.Store(ctx, targetName, c)
}

// Discard cred
func Discard(ctx context.Context, targetName string) error {
	return provider.Discard(ctx, targetName)
}
