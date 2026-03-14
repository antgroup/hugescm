//go:build windows

// Package keyring provides cross-platform credential storage for Zeta.
// This file implements the Windows backend using Windows Credential Manager.
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

	// CRED_MAX_STRING_LENGTH is the maximum length for string fields.
	CRED_MAX_STRING_LENGTH = 256

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

// Get retrieves credentials from Windows Credential Manager.
// Follows git-credential-manager pattern:
// - Uses CRED_TYPE_GENERIC
// - Target name format: "zeta+<protocol>://<server>[:<port>][<path>]"
// - Returns nil, ErrNotFound if credential doesn't exist
// Note: opts are ignored on Windows as the native credential manager is always used.
func Get(ctx context.Context, cred *Cred, opts ...Option) (*Cred, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if cred == nil {
		return nil, errors.New("credential cannot be nil")
	}

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
		return nil, errors.New("invalid credential: empty password")
	}

	password := windows.UTF16PtrToString((*uint16)(unsafe.Pointer(result.CredentialBlob)))

	// Validate password
	if password == "" {
		return nil, errors.New("invalid credential: empty password not allowed")
	}

	return &Cred{
		UserName: username,
		Password: password,
		Protocol: cred.Protocol,
		Server:   cred.Server,
		Port:     cred.Port,
		Path:     cred.Path,
	}, nil
}

// Store saves credentials in Windows Credential Manager.
// Follows git-credential-manager pattern:
// - Uses CRED_TYPE_GENERIC
// - Target name format: "zeta+<protocol>://<server>[:<port>][<path>]"
// - Stores username and password
// - If credential exists, it will be overwritten
// Note: opts are ignored on Windows as the native credential manager is always used.
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

	// Validate size limits
	if len(cred.UserName) > CRED_MAX_USERNAME_LENGTH {
		return fmt.Errorf("username too long (max %d bytes)", CRED_MAX_USERNAME_LENGTH)
	}

	targetName := buildTargetName(cred)
	if targetName == "" {
		return errors.New("invalid credential: target name cannot be empty")
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

	// Convert password to UTF-16
	passwordUTF16 := windows.StringToUTF16(cred.Password)

	// Prepare credential structure
	c := CREDENTIALW{
		Type:               CRED_TYPE_GENERIC,
		Persist:            CRED_PERSIST_LOCAL_MACHINE,
		TargetName:         targetNameUTF16,
		UserName:           userNameUTF16,
		CredentialBlobSize: uint32(len(passwordUTF16) * 2), // UTF-16: 2 bytes per character
		CredentialBlob:     (*byte)(unsafe.Pointer(&passwordUTF16[0])),
		Comment:            commentUTF16,
		Flags:              0,
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

// buildTargetName constructs a unique target name for Windows Credential Manager.
// Format: "zeta+<protocol>://<server>[:<port>][<path>]"
// This follows the pattern used by git-credential-manager for Windows.
func buildTargetName(cred *Cred) string {
	protocol := cred.Protocol
	if protocol == "" {
		protocol = "https"
	}

	target := fmt.Sprintf("zeta+%s://%s", protocol, cred.Server)

	if cred.Port != 0 {
		target += fmt.Sprintf(":%d", cred.Port)
	}

	if cred.Path != "" {
		target += cred.Path
	}

	return target
}

// Erase removes credentials from Windows Credential Manager.
// Follows git-credential-manager pattern:
// - Uses CRED_TYPE_GENERIC
// - Target name format: "zeta+<protocol>://<server>[:<port>][<path>]"
// - Returns nil if credential doesn't exist (no error)
// Note: opts are ignored on Windows as the native credential manager is always used.
func Erase(ctx context.Context, cred *Cred, opts ...Option) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if cred == nil {
		return errors.New("credential cannot be nil")
	}

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
		// Check if it's a "not found" error
		if errno, ok := err.(syscall.Errno); ok && errno == ERROR_NOT_FOUND {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	return nil
}
