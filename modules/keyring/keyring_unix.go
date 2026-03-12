//go:build (dragonfly && cgo) || (freebsd && cgo) || linux || netbsd || openbsd

// Package keyring provides cross-platform credential storage for Zeta.
// This file implements Unix/Linux backend using libsecret (Secret Service API).
package keyring

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ss "github.com/antgroup/hugescm/modules/keyring/secret_service"
	dbus "github.com/godbus/dbus/v5"
)

// Constants for Unix/Linux systems
const (
	// zetaUserName is the fixed username used for all stored credentials.
	// We use a constant username and encode the actual username in the credential data.
	zetaUserName = "zeta-credential-manager"

	// maxUnixUserNameLength is the maximum username length for Unix/Linux systems.
	// Matched with Windows CRED_MAX_USERNAME_LENGTH for consistency.
	maxUnixUserNameLength = 513

	// maxUnixPasswordLength is the maximum password length for Unix/Linux systems.
	// While there's no theoretical limit, performance suffers with big values (>100KiB).
	// We set a reasonable limit of 100KiB.
	maxUnixPasswordLength = 100 * 1024 // 100 KiB
)

// Get retrieves credentials from libsecret (Secret Service API).
// Follows git-credential-libsecret pattern:
// - Uses fixed username "zeta-credential-manager"
// - Encodes username and password as "username\0password"
// - Target name format: "zeta:<protocol>:<server>[:<port>][<path>]"
// Returns nil, ErrNotFound if credential doesn't exist
func Get(ctx context.Context, cred *Cred) (*Cred, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if cred == nil {
		return nil, errors.New("credential cannot be nil")
	}

	svc, err := ss.NewSecretService()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to secret service: %w", err)
	}

	targetName := buildUnixTargetName(cred)
	item, err := findItem(svc, targetName, zetaUserName)
	if err != nil {
		return nil, err
	}

	// Open a session to retrieve the secret
	session, err := svc.OpenSession()
	if err != nil {
		return nil, fmt.Errorf("failed to open session: %w", err)
	}
	defer svc.Close(session)

	// Unlock the item if it's locked
	if err := svc.Unlock(item); err != nil {
		return nil, fmt.Errorf("failed to unlock item: %w", err)
	}

	// Retrieve the secret
	secret, err := svc.GetSecret(item, session.Path())
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	// Parse the credential data (username + null byte + password)
	userName, password, ok := strings.Cut(string(secret.Value), "\x00")
	if !ok {
		return nil, errors.New("invalid credential format")
	}

	// Validate password
	if password == "" {
		return nil, errors.New("invalid credential: empty password not allowed")
	}

	// Return credential with all fields
	return &Cred{
		UserName: userName,
		Password: password,
		Protocol: cred.Protocol,
		Server:   cred.Server,
		Port:     cred.Port,
		Path:     cred.Path,
	}, nil
}

// Store saves credentials in libsecret (Secret Service API).
// Follows git-credential-libsecret pattern:
// - Uses fixed username "zeta-credential-manager"
// - Encodes username and password as "username\0password"
// - Target name format: "zeta:<protocol>:<server>[:<port>][<path>]"
// - If credential exists, it will be overwritten
func Store(ctx context.Context, cred *Cred) error {
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
	if len(cred.UserName) > maxUnixUserNameLength {
		return fmt.Errorf("username too long (max %d bytes)", maxUnixUserNameLength)
	}
	if len(cred.Password) > maxUnixPasswordLength {
		return fmt.Errorf("password too long (max %d bytes)", maxUnixPasswordLength)
	}

	svc, err := ss.NewSecretService()
	if err != nil {
		return fmt.Errorf("failed to connect to secret service: %w", err)
	}

	// Open a session
	session, err := svc.OpenSession()
	if err != nil {
		return fmt.Errorf("failed to open session: %w", err)
	}
	defer svc.Close(session)

	targetName := buildUnixTargetName(cred)

	// Build attributes for searching the credential
	attributes := map[string]string{
		"username": zetaUserName,
		"service":  targetName,
	}

	// Create secret object
	secret := ss.NewSecret(session.Path(), cred.Password)

	// Get login collection
	collection := svc.GetLoginCollection()

	// Unlock the collection
	if err := svc.Unlock(collection.Path()); err != nil {
		return fmt.Errorf("failed to unlock collection: %w", err)
	}

	// Encode credential data (username + null byte + password)
	body := fmt.Sprintf("%s\x00%s", cred.UserName, cred.Password)

	// Create or update the item
	secret.Value = []byte(body)

	// Try to create the item
	err = svc.CreateItem(
		collection,
		fmt.Sprintf("Zeta credential for %s", cred.Server),
		attributes,
		secret,
	)

	if err != nil {
		// Item might already exist, try to update it
		item, findErr := findItem(svc, targetName, zetaUserName)
		if findErr != nil {
			return fmt.Errorf("failed to create item: %w", err)
		}

		if err := svc.Delete(item); err != nil {
			return fmt.Errorf("failed to delete existing item: %w", err)
		}

		// Try creating again
		if err := svc.CreateItem(
			collection,
			fmt.Sprintf("Zeta credential for %s", cred.Server),
			attributes,
			secret,
		); err != nil {
			return fmt.Errorf("failed to create item after delete: %w", err)
		}
	}

	return nil
}

// Erase removes credentials from libsecret (Secret Service API).
// Follows git-credential-libsecret pattern:
// - Uses fixed username "zeta-credential-manager"
// - Target name format: "zeta:<protocol>:<server>[:<port>][<path>]"
// - Returns ErrNotFound if credential doesn't exist
func Erase(ctx context.Context, cred *Cred) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if cred == nil {
		return errors.New("credential cannot be nil")
	}

	svc, err := ss.NewSecretService()
	if err != nil {
		return fmt.Errorf("failed to connect to secret service: %w", err)
	}

	targetName := buildUnixTargetName(cred)
	item, err := findItem(svc, targetName, zetaUserName)
	if err != nil {
		return err
	}

	if err := svc.Delete(item); err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}

// findItem searches for an item in libsecret by service and username.
func findItem(svc *ss.SecretService, service, user string) (dbus.ObjectPath, error) {
	collection := svc.GetLoginCollection()

	search := map[string]string{
		"username": user,
		"service":  service,
	}

	if err := svc.Unlock(collection.Path()); err != nil {
		return "", fmt.Errorf("failed to unlock collection: %w", err)
	}

	results, err := svc.SearchItems(collection, search)
	if err != nil {
		return "", fmt.Errorf("failed to search items: %w", err)
	}

	if len(results) == 0 {
		return "", ErrNotFound
	}

	return results[0], nil
}

// buildUnixTargetName constructs a unique target name for libsecret.
// The format is: "zeta:<protocol>:<server>:<port>:<path>"
// This follows the pattern used by git-credential-libsecret for compatibility.
func buildUnixTargetName(cred *Cred) string {
	protocol := cred.Protocol
	if protocol == "" {
		protocol = "https"
	}

	target := fmt.Sprintf("zeta:%s:%s", protocol, cred.Server)

	if cred.Port != 0 {
		target += fmt.Sprintf(":%d", cred.Port)
	}

	if cred.Path != "" {
		target += cred.Path
	}

	return target
}
