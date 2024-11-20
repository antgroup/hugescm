//go:build windows

package keyring

import (
	"context"
	"syscall"

	"github.com/danieljoos/wincred"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type windowsKeychain struct{}

func init() {
	provider = windowsKeychain{}
}

// https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentiala
const (
	CRED_MAX_GENERIC_TARGET_NAME_LENGTH = 32767
	CRED_MAX_USERNAME_LENGTH            = 513
	CRED_MAX_CREDENTIAL_BLOB_SIZE       = 5 * 512
)

func (k windowsKeychain) Find(ctx context.Context, targetName string) (*Cred, error) {
	cred, err := wincred.GetGenericCredential(targetName)
	if err != nil {
		if err == syscall.ERROR_NOT_FOUND {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// https://github.com/danieljoos/wincred?tab=readme-ov-file#encoding
	dec := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
	password, _, err := transform.Bytes(dec, cred.CredentialBlob)
	if err != nil {
		return nil, err
	}
	return &Cred{UserName: cred.UserName, Password: string(password)}, nil
}

func (k windowsKeychain) Store(ctx context.Context, targetName string, c *Cred) error {

	// userName
	if len(c.UserName) > CRED_MAX_USERNAME_LENGTH {
		return ErrSetDataTooBig
	}

	// Password
	if len(c.Password) > CRED_MAX_CREDENTIAL_BLOB_SIZE {
		return ErrSetDataTooBig
	}

	// TargetName: https://zeta.example.io
	if len(targetName) > CRED_MAX_GENERIC_TARGET_NAME_LENGTH {
		return ErrSetDataTooBig
	}
	// https://github.com/danieljoos/wincred?tab=readme-ov-file#encoding
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	encodedCred, _, err := transform.Bytes(encoder, []byte(c.Password))
	if err != nil {
		return err
	}
	cred := wincred.NewGenericCredential(targetName)
	cred.UserName = c.UserName
	cred.CredentialBlob = encodedCred
	return cred.Write()
}

func (k windowsKeychain) Discard(ctx context.Context, targetName string) error {
	cred, err := wincred.GetGenericCredential(targetName)
	if err != nil {
		if err == syscall.ERROR_NOT_FOUND {
			return ErrNotFound
		}
		return err
	}

	return cred.Delete()
}
