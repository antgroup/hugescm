//go:build darwin

// Package keyring provides cross-platform credential storage for Zeta.
// This file implements the macOS (Darwin) backend using purego without CGO.
// Default: Uses Security.framework via purego (recommended)
// Alternative: Set storage="security" to use /usr/bin/security CLI tool
// Alternative: Set storage="file" to use encrypted file storage
package keyring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Core Foundation and Security framework constants
const (
	kCFStringEncodingUTF8 = 0x08000100
	kCFAllocatorDefault   = 0
)

type osStatus int32

const (
	errSecSuccess       osStatus = 0      // No error.
	errSecDuplicateItem osStatus = -25299 // The specified item already exists in the keychain.
	errSecItemNotFound  osStatus = -25300 // The specified item could not be found in the keychain.
)

type _CFRange struct {
	location int64
	length   int64
}

type _CFNumberType int64 // CFNumberType is alias for CFIndex, which is int64 on 64-bit systems

const (
	// CFNumber type constants for number conversion
	kCFNumberIntType _CFNumberType = 3 // SInt32Type
)

var (
	kCFTypeDictionaryKeyCallBacks   uintptr
	kCFTypeDictionaryValueCallBacks uintptr
	kCFBooleanTrue                  uintptr
)

var (
	kSecClass                         uintptr
	kSecClassInternetPassword         uintptr
	kSecAttrServer                    uintptr
	kSecAttrAccount                   uintptr
	kSecAttrProtocol                  uintptr
	kSecAttrProtocolHTTP              uintptr
	kSecAttrProtocolHTTPS             uintptr
	kSecAttrProtocolFTP               uintptr
	kSecAttrProtocolFTPS              uintptr
	kSecAttrProtocolIMAP              uintptr
	kSecAttrProtocolIMAPS             uintptr
	kSecAttrProtocolSMTP              uintptr
	kSecAttrPort                      uintptr
	kSecAttrPath                      uintptr
	kSecAttrAuthenticationType        uintptr
	kSecAttrAuthenticationTypeDefault uintptr
	kSecValueData                     uintptr
	kSecReturnData                    uintptr
	kSecReturnAttributes              uintptr
	kSecMatchLimit                    uintptr
	kSecMatchLimitAll                 uintptr
)

var (
	CFDictionaryCreate        func(allocator uintptr, keys, values *uintptr, numValues int64, keyCallBacks, valueCallBacks uintptr) uintptr
	CFStringCreateWithCString func(allocator uintptr, cStr string, encoding uint32) uintptr
	CFDataCreate              func(alloc uintptr, bytes []byte, length int64) uintptr
	CFDataGetLength           func(theData uintptr) int64
	CFDataGetBytes            func(theData uintptr, range_ _CFRange, buffer []byte)
	CFRelease                 func(cf uintptr)
	CFNumberCreate            func(allocator uintptr, theType _CFNumberType, valuePtr uintptr) uintptr
)

var (
	SecItemCopyMatching  func(query uintptr, result *uintptr) osStatus
	SecItemAdd           func(query uintptr, result uintptr) osStatus
	SecItemUpdate        func(query uintptr, attributesToUpdate uintptr) osStatus
	SecItemDelete        func(query uintptr) osStatus
	CFDictionaryGetValue func(theDict uintptr, key uintptr) uintptr
	CFStringGetCString   func(theString uintptr, buffer *byte, bufferSize int64, encoding uint32) int64
	CFStringGetLength    func(theString uintptr) int64
)

var (
	puregoOnce sync.Once
	puregoErr  error
)

// ensureInitialized ensures the keyring is initialized.
// It uses sync.Once to ensure initialization happens only once.
// Returns an error if initialization fails.
func ensureInitialized() error {
	puregoOnce.Do(func() {
		puregoErr = initializeKeyring()
	})
	return puregoErr
}

// initializeKeyring initializes the PureGo bindings for macOS Security framework.
func initializeKeyring() error {
	cfLib, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("failed to load CoreFoundation framework: %w", err)
	}

	// Load CoreFoundation constants
	ptr, err := purego.Dlsym(cfLib, "kCFTypeDictionaryKeyCallBacks")
	if err != nil {
		return fmt.Errorf("failed to load kCFTypeDictionaryKeyCallBacks: %w", err)
	}
	kCFTypeDictionaryKeyCallBacks = deref(ptr)

	ptr, err = purego.Dlsym(cfLib, "kCFTypeDictionaryValueCallBacks")
	if err != nil {
		return fmt.Errorf("failed to load kCFTypeDictionaryValueCallBacks: %w", err)
	}
	kCFTypeDictionaryValueCallBacks = deref(ptr)

	ptr, err = purego.Dlsym(cfLib, "kCFBooleanTrue")
	if err != nil {
		return fmt.Errorf("failed to load kCFBooleanTrue: %w", err)
	}
	kCFBooleanTrue = deref(ptr)

	purego.RegisterLibFunc(&CFDictionaryCreate, cfLib, "CFDictionaryCreate")
	purego.RegisterLibFunc(&CFStringCreateWithCString, cfLib, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&CFDataCreate, cfLib, "CFDataCreate")
	purego.RegisterLibFunc(&CFDataGetLength, cfLib, "CFDataGetLength")
	purego.RegisterLibFunc(&CFDataGetBytes, cfLib, "CFDataGetBytes")
	purego.RegisterLibFunc(&CFRelease, cfLib, "CFRelease")
	purego.RegisterLibFunc(&CFNumberCreate, cfLib, "CFNumberCreate")

	secLib, err := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("failed to load Security framework: %w", err)
	}

	// Load Security constants
	symbols := []struct {
		sym  string
		addr *uintptr
	}{
		{"kSecClass", &kSecClass},
		{"kSecClassInternetPassword", &kSecClassInternetPassword},
		{"kSecAttrServer", &kSecAttrServer},
		{"kSecAttrAccount", &kSecAttrAccount},
		{"kSecAttrProtocol", &kSecAttrProtocol},
		{"kSecAttrProtocolHTTP", &kSecAttrProtocolHTTP},
		{"kSecAttrProtocolHTTPS", &kSecAttrProtocolHTTPS},
		{"kSecAttrProtocolFTP", &kSecAttrProtocolFTP},
		{"kSecAttrProtocolFTPS", &kSecAttrProtocolFTPS},
		{"kSecAttrProtocolIMAP", &kSecAttrProtocolIMAP},
		{"kSecAttrProtocolIMAPS", &kSecAttrProtocolIMAPS},
		{"kSecAttrProtocolSMTP", &kSecAttrProtocolSMTP},
		{"kSecAttrPort", &kSecAttrPort},
		{"kSecAttrPath", &kSecAttrPath},
		{"kSecAttrAuthenticationType", &kSecAttrAuthenticationType},
		{"kSecAttrAuthenticationTypeDefault", &kSecAttrAuthenticationTypeDefault},
		{"kSecValueData", &kSecValueData},
		{"kSecReturnData", &kSecReturnData},
		{"kSecReturnAttributes", &kSecReturnAttributes},
		{"kSecMatchLimit", &kSecMatchLimit},
		{"kSecMatchLimitAll", &kSecMatchLimitAll},
	}

	for _, s := range symbols {
		ptr, err := purego.Dlsym(secLib, s.sym)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", s.sym, err)
		}
		*s.addr = deref(ptr)
	}

	purego.RegisterLibFunc(&SecItemCopyMatching, secLib, "SecItemCopyMatching")
	purego.RegisterLibFunc(&SecItemAdd, secLib, "SecItemAdd")
	purego.RegisterLibFunc(&SecItemUpdate, secLib, "SecItemUpdate")
	purego.RegisterLibFunc(&SecItemDelete, secLib, "SecItemDelete")
	purego.RegisterLibFunc(&CFDictionaryGetValue, cfLib, "CFDictionaryGetValue")
	purego.RegisterLibFunc(&CFStringGetCString, cfLib, "CFStringGetCString")
	purego.RegisterLibFunc(&CFStringGetLength, cfLib, "CFStringGetLength")

	return nil
}

// Get retrieves credentials from the configured storage backend.
// Default uses Security.framework via purego.
// Set opts storage="security" to use /usr/bin/security CLI.
// Set opts storage="file" to use encrypted file storage.
func Get(ctx context.Context, cred *Cred, opts ...Option) (*Cred, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if cred == nil {
		return nil, errors.New("credential cannot be nil")
	}
	options := resolveStorageOptions(opts...)
	switch options.Storage {
	case storageAuto:
		return getFromKeychain(ctx, cred)
	case storageSecurity:
		return getFromSecurityCLI(ctx, cred)
	case storageFile:
		storage, err := newCredentialStorage(options.EncryptionKey, options.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize file storage: %w", err)
		}
		return storage.Get(ctx, cred)
	case storageNone:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("unknown storage mode: %s", options.Storage)
	}
}

// getFromKeychain retrieves credentials using Security.framework via purego.
func getFromKeychain(ctx context.Context, cred *Cred) (*Cred, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if err := ensureInitialized(); err != nil {
		return nil, fmt.Errorf("failed to initialize keyring: %w", err)
	}

	if cred.Server == "" {
		return nil, errors.New("server is required")
	}

	cfServer := CFStringCreateWithCString(kCFAllocatorDefault, cred.Server, kCFStringEncodingUTF8)
	defer CFRelease(cfServer)

	// Build query following git-credential-osxkeychain pattern:
	// Use kSecClassInternetPassword, kSecAttrServer as base
	// Add optional fields: kSecAttrProtocol, kSecAttrAccount, kSecAttrPath, kSecAttrPort
	// Add kSecReturnAttributes and kSecReturnData to get both metadata and password
	keys := []uintptr{
		kSecClass,
		kSecAttrServer,
		kSecReturnAttributes,
		kSecReturnData,
	}
	values := []uintptr{
		kSecClassInternetPassword,
		cfServer,
		kCFBooleanTrue,
		kCFBooleanTrue,
	}

	// Add optional fields and track CF objects for cleanup
	optionalFields := newOptionalFields(cred, &keys, &values)
	defer optionalFields.Release()

	// Add authentication type (required for git-credential-osxkeychain compatibility)
	keys = append(keys, kSecAttrAuthenticationType)
	values = append(values, kSecAttrAuthenticationTypeDefault)

	query := CFDictionaryCreate(
		kCFAllocatorDefault,
		&keys[0], &values[0], int64(len(keys)),
		kCFTypeDictionaryKeyCallBacks,
		kCFTypeDictionaryValueCallBacks,
	)
	defer CFRelease(query)

	var result uintptr
	st := SecItemCopyMatching(query, &result)
	if st == errSecItemNotFound {
		return nil, ErrNotFound
	}
	if st != errSecSuccess {
		return nil, fmt.Errorf("error SecItemCopyMatching: %d", st)
	}
	defer CFRelease(result)

	// Extract username from result
	accountValue := CFDictionaryGetValue(result, kSecAttrAccount)
	username := ""
	if accountValue != 0 {
		// CFStringGetLength returns UTF-16 code units, but CFStringGetCString needs UTF-8 buffer.
		// UTF-8 can use up to 4 bytes per character, so allocate 4x the UTF-16 length.
		if length := CFStringGetLength(accountValue); length > 0 {
			buffer := make([]byte, length*4+1)
			if CFStringGetCString(accountValue, &buffer[0], int64(len(buffer)), kCFStringEncodingUTF8) == 0 {
				return nil, errors.New("failed to convert username to UTF-8")
			}
			username = strings.TrimRight(string(buffer), "\x00")
		}
	}

	// Extract password from result
	passwordValue := CFDictionaryGetValue(result, kSecValueData)
	password := ""
	if passwordValue != 0 {
		length := CFDataGetLength(passwordValue)
		if length > 0 {
			buffer := make([]byte, length)
			CFDataGetBytes(passwordValue, _CFRange{0, length}, buffer)
			password = string(buffer)
		}
	}

	return &Cred{
		UserName: username,
		Password: password,
		Protocol: cred.Protocol,
		Server:   cred.Server,
		Path:     cred.Path,
		Port:     cred.Port,
	}, nil
}

// Store saves credentials to the configured storage backend.
// Default uses Security.framework via purego.
// Set opts storage="security" to use /usr/bin/security CLI.
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

	options := resolveStorageOptions(opts...)
	switch options.Storage {
	case storageAuto:
		return storeToKeychain(ctx, cred)
	case storageSecurity:
		return storeToSecurityCLI(ctx, cred)
	case storageFile:
		storage, err := newCredentialStorage(options.EncryptionKey, options.StoragePath)
		if err != nil {
			return fmt.Errorf("failed to initialize file storage: %w", err)
		}
		return storage.Store(ctx, cred)
	case storageNone:
		return ErrStorageDisabled
	default:
		return fmt.Errorf("unknown storage mode: %s", options.Storage)
	}
}

// storeToKeychain stores credentials using Security.framework via purego.
func storeToKeychain(ctx context.Context, cred *Cred) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := ensureInitialized(); err != nil {
		return fmt.Errorf("failed to initialize keyring: %w", err)
	}

	cfServer := CFStringCreateWithCString(kCFAllocatorDefault, cred.Server, kCFStringEncodingUTF8)
	defer CFRelease(cfServer)
	cfPasswordData := CFDataCreate(kCFAllocatorDefault, []byte(cred.Password), int64(len(cred.Password)))
	defer CFRelease(cfPasswordData)

	// Build attributes following git-credential-osxkeychain pattern:
	// Always include: kSecClass, kSecAttrServer, kSecAttrAccount, kSecAttrProtocol, kSecAttrAuthenticationType
	// Optionally include: kSecAttrPath, kSecAttrPort
	// Then update with: kSecValueData
	keys := []uintptr{
		kSecClass,
		kSecAttrServer,
		kSecValueData,
	}
	values := []uintptr{
		kSecClassInternetPassword,
		cfServer,
		cfPasswordData,
	}

	// Add optional fields and track CF objects for cleanup
	optionalFields := newOptionalFields(cred, &keys, &values)
	defer optionalFields.Release()

	// Add authentication type (required for git-credential-osxkeychain compatibility)
	keys = append(keys, kSecAttrAuthenticationType)
	values = append(values, kSecAttrAuthenticationTypeDefault)

	query := CFDictionaryCreate(
		kCFAllocatorDefault,
		&keys[0], &values[0], int64(len(keys)),
		kCFTypeDictionaryKeyCallBacks,
		kCFTypeDictionaryValueCallBacks,
	)
	defer CFRelease(query)

	sa := SecItemAdd(query, 0)
	if sa == errSecSuccess {
		return nil
	}

	if sa != errSecDuplicateItem {
		return fmt.Errorf("error SecItemAdd: %d", sa)
	}

	// Build update query matching same criteria as add query
	updateKeys := []uintptr{kSecClass, kSecAttrServer}
	updateValues := []uintptr{kSecClassInternetPassword, cfServer}

	// Add optional fields and track CF objects for cleanup
	updateOptionalFields := newOptionalFields(cred, &updateKeys, &updateValues)
	defer updateOptionalFields.Release()

	// Add authentication type (required for git-credential-osxkeychain compatibility)
	updateKeys = append(updateKeys, kSecAttrAuthenticationType)
	updateValues = append(updateValues, kSecAttrAuthenticationTypeDefault)

	updateQuery := CFDictionaryCreate(
		kCFAllocatorDefault,
		&updateKeys[0], &updateValues[0], int64(len(updateKeys)),
		kCFTypeDictionaryKeyCallBacks,
		kCFTypeDictionaryValueCallBacks,
	)
	defer CFRelease(updateQuery)

	// Build attributes to update (only password)
	attrsToUpdateKeys := []uintptr{kSecValueData}
	attrsToUpdateValues := []uintptr{cfPasswordData}
	attrsToUpdate := CFDictionaryCreate(
		kCFAllocatorDefault,
		&attrsToUpdateKeys[0], &attrsToUpdateValues[0], int64(len(attrsToUpdateKeys)),
		kCFTypeDictionaryKeyCallBacks,
		kCFTypeDictionaryValueCallBacks,
	)
	defer CFRelease(attrsToUpdate)

	su := SecItemUpdate(updateQuery, attrsToUpdate)
	if su != errSecSuccess {
		return fmt.Errorf("error SecItemUpdate: %d", su)
	}
	return nil
}

// Erase removes credentials from the configured storage backend.
// Default uses Security.framework via purego.
// Set opts storage="security" to use /usr/bin/security CLI.
// Set opts storage="file" to use encrypted file storage.
func Erase(ctx context.Context, cred *Cred, opts ...Option) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if cred == nil {
		return errors.New("credential cannot be nil")
	}

	options := resolveStorageOptions(opts...)
	switch options.Storage {
	case storageAuto:
		return eraseFromKeychain(ctx, cred)
	case storageSecurity:
		return eraseFromSecurityCLI(ctx, cred)
	case storageFile:
		storage, err := newCredentialStorage(options.EncryptionKey, options.StoragePath)
		if err != nil {
			return fmt.Errorf("failed to initialize file storage: %w", err)
		}
		return storage.Erase(ctx, cred)
	case storageNone:
		return ErrStorageDisabled
	default:
		return fmt.Errorf("unknown storage mode: %s", options.Storage)
	}
}

// eraseFromKeychain removes credentials using Security.framework via purego.
func eraseFromKeychain(ctx context.Context, cred *Cred) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := ensureInitialized(); err != nil {
		return fmt.Errorf("failed to initialize keyring: %w", err)
	}

	// Use server from cred
	server := cred.Server
	if server == "" {
		return errors.New("server is required")
	}

	cfServer := CFStringCreateWithCString(kCFAllocatorDefault, server, kCFStringEncodingUTF8)
	defer CFRelease(cfServer)

	// Build query following git-credential-osxkeychain pattern:
	// Use kSecClass, kSecAttrServer as base
	// Add optional fields: kSecAttrProtocol, kSecAttrAccount, kSecAttrPath, kSecAttrPort
	// Use kSecMatchLimit=kSecMatchLimitAll to delete all matching credentials
	keys := []uintptr{
		kSecClass,
		kSecAttrServer,
		kSecMatchLimit, // Key for match limit
	}
	values := []uintptr{
		kSecClassInternetPassword,
		cfServer,
		kSecMatchLimitAll, // Value: delete all matching credentials
	}

	// Add optional fields and track CF objects for cleanup
	optionalFields := newOptionalFields(cred, &keys, &values)
	defer optionalFields.Release()

	// Add authentication type (required for git-credential-osxkeychain compatibility)
	keys = append(keys, kSecAttrAuthenticationType)
	values = append(values, kSecAttrAuthenticationTypeDefault)

	query := CFDictionaryCreate(
		kCFAllocatorDefault,
		&keys[0], &values[0], int64(len(keys)),
		kCFTypeDictionaryKeyCallBacks,
		kCFTypeDictionaryValueCallBacks,
	)
	defer CFRelease(query)

	st := SecItemDelete(query)
	if st == errSecItemNotFound {
		return ErrNotFound
	}
	if st != errSecSuccess {
		return fmt.Errorf("error SecItemDelete: %d", st)
	}
	return nil
}

// darwinProtocolFromScheme converts protocol string to keychain protocol constant.
func darwinProtocolFromScheme(protocol string) uintptr {
	switch strings.ToLower(protocol) {
	case "https":
		return kSecAttrProtocolHTTPS
	case "http":
		return kSecAttrProtocolHTTP
	case "ftp":
		return kSecAttrProtocolFTP
	case "ftps":
		return kSecAttrProtocolFTPS
	case "imap":
		return kSecAttrProtocolIMAP
	case "imaps":
		return kSecAttrProtocolIMAPS
	case "smtp":
		return kSecAttrProtocolSMTP
	default:
		return 0 // Unknown protocol
	}
}

// ========== Helper Functions ==========

// darwinOptionalFields holds optional CF objects that may be added to queries.
type darwinOptionalFields struct {
	cfProtocol uintptr
	cfAccount  uintptr
	cfPath     uintptr
	cfPort     uintptr
}

// Release releases all CF objects held by darwinOptionalFields.
// Note: cfProtocol is a constant value, not a CF object, so it's not released.
func (f *darwinOptionalFields) Release() {
	if f.cfAccount != 0 {
		CFRelease(f.cfAccount)
		f.cfAccount = 0
	}
	if f.cfPath != 0 {
		CFRelease(f.cfPath)
		f.cfPath = 0
	}
	if f.cfPort != 0 {
		CFRelease(f.cfPort)
		f.cfPort = 0
	}
}

// newOptionalFields creates and returns darwinOptionalFields with optional credential fields.
// It appends the fields to the provided keys and values slices.
// The caller should call fields.Release() when no longer needed.
func newOptionalFields(cred *Cred, keys, values *[]uintptr) *darwinOptionalFields {
	fields := &darwinOptionalFields{}

	// Add protocol if specified
	if cred.Protocol != "" {
		if protocol := darwinProtocolFromScheme(cred.Protocol); protocol != 0 {
			fields.cfProtocol = protocol
			*keys = append(*keys, kSecAttrProtocol)
			*values = append(*values, protocol)
		}
	}

	// Add username if specified
	if cred.UserName != "" {
		fields.cfAccount = CFStringCreateWithCString(kCFAllocatorDefault, cred.UserName, kCFStringEncodingUTF8)
		*keys = append(*keys, kSecAttrAccount)
		*values = append(*values, fields.cfAccount)
	}

	// Add path if specified
	if cred.Path != "" {
		fields.cfPath = CFStringCreateWithCString(kCFAllocatorDefault, cred.Path, kCFStringEncodingUTF8)
		*keys = append(*keys, kSecAttrPath)
		*values = append(*values, fields.cfPath)
	}

	// Add port if specified
	// Use int32 (kCFNumberIntType) to support full port range 0-65535
	// int16 can only hold 0-32767 which is insufficient
	if cred.Port != 0 {
		portInt32 := int32(cred.Port)
		fields.cfPort = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, uintptr(unsafe.Pointer(&portInt32)))
		*keys = append(*keys, kSecAttrPort)
		*values = append(*values, fields.cfPort)
	}

	return fields
}

// deref dereferences a uintptr that points to another uintptr.
// This is used to load values from symbol addresses returned by Dlsym.
// For example, Dlsym returns the address of kCFBooleanTrue, which itself
// contains the actual CFBooleanRef value.
func deref(ptr uintptr) uintptr {
	return **(**uintptr)(unsafe.Pointer(&ptr))
}
