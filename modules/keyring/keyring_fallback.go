package keyring

import (
	"context"
	"errors"
	"runtime"
)

// All of the following methods error out on unsupported platforms
var ErrUnsupportedPlatform = errors.New("unsupported platform: " + runtime.GOOS)

type fallbackServiceProvider struct{}

func (fallbackServiceProvider) Find(ctx context.Context, targetName string) (*Cred, error) {
	return nil, ErrUnsupportedPlatform
}

func (fallbackServiceProvider) Store(ctx context.Context, targetName string, c *Cred) error {
	return ErrUnsupportedPlatform
}

func (fallbackServiceProvider) Discard(ctx context.Context, targetName string) error {
	return ErrUnsupportedPlatform
}
