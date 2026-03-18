//go:build windows

// Package keyring provides cross-platform credential storage for Zeta.
// This file implements the Windows backend using Windows Credential Manager.
// Default: Uses Windows Credential Manager API
// Alternative: Set storage="file" to use encrypted file storage
package keyring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Constants for Windows Credential Manager
const (
	// CRED_MAX_USERNAME_LENGTH is the maximum username length in Windows.
	// Source: https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentiala
	CRED_MAX_USERNAME_LENGTH = 513

	// CRED_MAX_GENERIC_TARGET_NAME_LENGTH is the maximum target name length.
	CRED_MAX_GENERIC_TARGET_NAME_LENGTH = 32767

	// CRED_MAX_CREDENTIAL_BLOB_SIZE Maximum size of CredentialBlob in bytes.
	CRED_MAX_CREDENTIAL_BLOB_SIZE = 512

	// CRED_TYPE_GENERIC is the credential type for generic credentials.
	CRED_TYPE_GENERIC = 1

	// CRED_PERSIST_LOCAL_MACHINE stores the credential in the local machine.
	CRED_PERSIST_LOCAL_MACHINE = 2

	// CRED_PERSIST_SESSION stores the credential for the session only.
	CRED_PERSIST_SESSION = 1
)

// Windows API constants
var (
	// advapi32.dll functions
	modadvapi32     = windows.NewLazySystemDLL("advapi32.dll")
	procCredWriteW  = modadvapi32.NewProc("CredWriteW")
	procCredReadW   = modadvapi32.NewProc("CredReadW")
	procCredDeleteW = modadvapi32.NewProc("CredDeleteW")
	procCredFree    = modadvapi32.NewProc("CredFree")

	// Error codes
	ERROR_NOT_FOUND = syscall.Errno(1168) // ERROR_NOT_FOUND
)

// CREDENTIALW is the Windows credential structure.
// Source: https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw
type CREDENTIALW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// Get retrieves credentials from the configured storage backend.
// Default uses Windows Credential Manager.
// Set opts storage="file" to use encrypted file storage.
func Get(ctx context.Context, cred *Cred, opts ...Option) (*Cred, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if cred == nil {
		return nil, errors.New("credential cannot be nil")
	}

	mode := resolveStorageMode(opts...)

	switch mode {
	case storageAuto:
		return getFromCredMan(ctx, cred)
	case storageFile:
		options := applyOptions(opts...)
		storage, err := newCredentialStorage(options.EncryptionKey, options.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize file storage: %w", err)
		}
		return storage.Get(ctx, cred)
	case storageNone:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("unknown storage mode: %s", mode)
	}
}

// getFromCredMan retrieves credentials using Windows Credential Manager.
func getFromCredMan(ctx context.Context, cred *Cred) (*Cred, error) {

	targetName := buildTargetName(cred)
	if targetName == "" {
		return nil, errors.New("invalid credential: target name cannot be empty")
	}

	// Convert target name to UTF-16
	targetNameUTF16, err := windows.UTF16PtrFromString(targetName)
	if err != nil {
		return nil, fmt.Errorf("failed to convert target name to UTF-16: %w", err)
	}

	// Prepare credential buffer
	var result *CREDENTIALW

	// Read credential
	ret, _, err := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetNameUTF16)),
		CRED_TYPE_GENERIC,
		0, // Flags
		uintptr(unsafe.Pointer(&result)),
	)
	if ret == 0 {
		// Windows syscall returns errno as err, check it explicitly
		if errno, ok := err.(syscall.Errno); ok && errno == ERROR_NOT_FOUND {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read credential: %w", err)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(result)))

	// Extract username
	username := cred.UserName
	if result.UserName != nil {
		username = windows.UTF16PtrToString(result.UserName)
	}

	// Extract password
	if result.CredentialBlob == nil || result.CredentialBlobSize == 0 {
		return nil, errors.New("password cannot be empty")
	}

	passwordRaw := unsafe.Slice(result.CredentialBlob, result.CredentialBlobSize)
	password := string(passwordRaw)

	return &Cred{
		UserName: username,
		Password: password,
		Protocol: cred.Protocol,
		Server:   cred.Server,
		Port:     cred.Port,
		Path:     cred.Path,
	}, nil
}

// Store saves credentials to the configured storage backend.
// Default uses Windows Credential Manager.
// Set opts storage="file" to use encrypted file storage.
func Store(ctx context.Context, cred *Cred, opts ...Option) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if cred == nil {
		return errors.New("credential cannot be nil")
	}

	// Validate input
	if cred.UserName == "" {
		return errors.New("username cannot be empty")
	}
	if cred.Password == "" {
		return errors.New("password cannot be empty")
	}
	if cred.Server == "" {
		return errors.New("server cannot be empty")
	}

	// Validate username cannot contain null byte
	if strings.Contains(cred.UserName, "\x00") {
		return errors.New("invalid username: contains null byte")
	}

	mode := resolveStorageMode(opts...)

	switch mode {
	case storageAuto:
		return storeToCredMan(ctx, cred)
	case storageFile:
		options := applyOptions(opts...)
		storage, err := newCredentialStorage(options.EncryptionKey, options.StoragePath)
		if err != nil {
			return fmt.Errorf("failed to initialize file storage: %w", err)
		}
		return storage.Store(ctx, cred)
	case storageNone:
		return ErrStorageDisabled
	default:
		return fmt.Errorf("unknown storage mode: %s", mode)
	}
}

// storeToCredMan stores credentials using Windows Credential Manager.
func storeToCredMan(ctx context.Context, cred *Cred) error {
	// Validate size limits
	if len(cred.UserName) > CRED_MAX_USERNAME_LENGTH {
		return fmt.Errorf("username too long (max %d bytes)", CRED_MAX_USERNAME_LENGTH)
	}

	targetName := buildTargetName(cred)
	if targetName == "" {
		return errors.New("invalid credential: target name cannot be empty")
	}

	// Validate target name length
	if len(targetName) > CRED_MAX_GENERIC_TARGET_NAME_LENGTH {
		return fmt.Errorf("target name too long (max %d bytes)", CRED_MAX_GENERIC_TARGET_NAME_LENGTH)
	}

	// Convert target name and username to UTF-16
	targetNameUTF16, err := windows.UTF16PtrFromString(targetName)
	if err != nil {
		return fmt.Errorf("failed to convert target name to UTF-16: %w", err)
	}

	userNameUTF16, err := windows.UTF16PtrFromString(cred.UserName)
	if err != nil {
		return fmt.Errorf("failed to convert username to UTF-16: %w", err)
	}

	commentStr := fmt.Sprintf("Zeta credential for %s", cred.Server)
	commentUTF16, err := windows.UTF16PtrFromString(commentStr)
	if err != nil {
		return fmt.Errorf("failed to convert comment to UTF-16: %w", err)
	}

	password := []byte(cred.Password)

	if len(password) > CRED_MAX_CREDENTIAL_BLOB_SIZE {
		return fmt.Errorf("password too long (max %d bytes)", CRED_MAX_CREDENTIAL_BLOB_SIZE)
	}

	// Prepare credential structure
	c := CREDENTIALW{
		Type:               CRED_TYPE_GENERIC,
		Persist:            CRED_PERSIST_LOCAL_MACHINE,
		TargetName:         targetNameUTF16,
		UserName:           userNameUTF16,
		CredentialBlobSize: uint32(len(password)),
		Comment:            commentUTF16,
	}

	if len(password) > 0 {
		c.CredentialBlob = &password[0]
	}

	// Write credential
	ret, _, err := procCredWriteW.Call(
		uintptr(unsafe.Pointer(&c)),
		0, // Flags
	)
	if ret == 0 {
		return fmt.Errorf("failed to write credential: %w", err)
	}

	return nil
}

// Erase removes credentials from the configured storage backend.
// Default uses Windows Credential Manager.
// Set opts storage="file" to use encrypted file storage.
func Erase(ctx context.Context, cred *Cred, opts ...Option) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if cred == nil {
		return errors.New("credential cannot be nil")
	}

	mode := resolveStorageMode(opts...)

	switch mode {
	case storageAuto:
		return eraseFromCredMan(ctx, cred)
	case storageFile:
		options := applyOptions(opts...)
		storage, err := newCredentialStorage(options.EncryptionKey, options.StoragePath)
		if err != nil {
			return fmt.Errorf("failed to initialize file storage: %w", err)
		}
		return storage.Erase(ctx, cred)
	case storageNone:
		return ErrStorageDisabled
	default:
		return fmt.Errorf("unknown storage mode: %s", mode)
	}
}

// eraseFromCredMan removes credentials using Windows Credential Manager.
func eraseFromCredMan(_ context.Context, cred *Cred) error {
	targetName := buildTargetName(cred)
	if targetName == "" {
		return errors.New("invalid credential: target name cannot be empty")
	}

	// Convert target name to UTF-16
	targetNameUTF16, err := windows.UTF16PtrFromString(targetName)
	if err != nil {
		return fmt.Errorf("failed to convert target name to UTF-16: %w", err)
	}

	// Delete credential
	ret, _, err := procCredDeleteW.Call(
		uintptr(unsafe.Pointer(targetNameUTF16)),
		CRED_TYPE_GENERIC,
		0, // Flags
	)
	if ret == 0 {
		// Windows syscall returns errno as err, check it explicitly
		if errno, ok := err.(syscall.Errno); ok && errno == ERROR_NOT_FOUND {
			return nil
		}
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	return nil
}
