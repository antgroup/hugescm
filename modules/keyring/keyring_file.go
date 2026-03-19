// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows

package keyring

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/base58"
	"golang.org/x/crypto/hkdf"
)

// credentialStorage implements encrypted file-based credential storage.
// Credentials are stored in TOML format with each field encrypted separately.
type credentialStorage struct {
	mu          sync.Mutex
	configDir   string
	key         []byte
	storagePath string
}

// credentialEntry represents a single encrypted credential entry in TOML
type credentialEntry struct {
	Target   string `toml:"target"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// credentialsFile represents the TOML file structure
type credentialsFile struct {
	Credentials []credentialEntry `toml:"credentials"`
}

const (
	defaultCredentialsFileName = "credentials"
	nonceSize                  = 12
)

// newCredentialStorage creates a new file-based credential storage.
// If encryptionKey is empty, it will be automatically derived from system information.
func newCredentialStorage(encryptionKey, storagePath string) (*credentialStorage, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	key, err := deriveOrValidateKey(encryptionKey)
	if err != nil {
		return nil, err
	}

	// Set storage path
	if storagePath == "" {
		storagePath = filepath.Join(configDir, defaultCredentialsFileName)
	}

	return &credentialStorage{
		configDir:   configDir,
		key:         key,
		storagePath: storagePath,
	}, nil
}

// deriveOrValidateKey derives or validates the encryption key.
// Supports: raw string, base58-encoded, or auto-derived.
func deriveOrValidateKey(encryptionKey string) ([]byte, error) {
	if encryptionKey == "" {
		return deriveEncryptionKey()
	}

	// Try base58 first (project standard)
	if keyBytes := base58.Decode(encryptionKey); len(keyBytes) > 0 {
		if !slices.Contains([]int{16, 24, 32}, len(keyBytes)) {
			return nil, fmt.Errorf("encryption key must be 16, 24, or 32 bytes (got %d)", len(keyBytes))
		}
		// Use HKDF to derive a 32-byte key for AES-256
		// This preserves the full entropy of shorter keys (16 or 24 bytes)
		// rather than zero-padding which reduces effective security.
		if len(keyBytes) < 32 {
			derived := make([]byte, 32)
			kdf := hkdf.New(sha256.New, keyBytes, nil, []byte("zeta-keyring-v1"))
			if _, err := io.ReadFull(kdf, derived); err != nil {
				return nil, fmt.Errorf("failed to derive key: %w", err)
			}
			return derived, nil
		}
		return keyBytes, nil
	}

	// Fallback: hash the raw string
	return hashKey(encryptionKey), nil
}

// hashKey hashes a raw string to a 32-byte key
func hashKey(key string) []byte {
	h := sha256.New()
	h.Write([]byte(key))
	return h.Sum(nil)
}

// getConfigDir returns the configuration directory path
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ".config", "zeta")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return configDir, nil
}

// deriveEncryptionKey derives an AES-256 key from system-specific information.
// Key = SHA-256(home_dir || hostname || username)
//
// SECURITY WARNING: This provides obfuscation-level protection, NOT cryptographic security.
// The key is derived from publicly accessible system information (home directory, hostname,
// username), which can be easily obtained by an attacker with local access. This prevents
// casual snooping but NOT a determined attacker.
//
// For production use requiring real security, provide an explicit encryption key via
// WithEncryptionKey() option, stored securely (e.g., hardware security module, secure
// enclave, or user-provided passphrase through a KDF like Argon2 or scrypt).
func deriveEncryptionKey() ([]byte, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	username := "unknown"
	if currentUser, err := user.Current(); err == nil {
		username = currentUser.Username
	}

	h := sha256.New()
	h.Write([]byte(homeDir))
	h.Write([]byte(hostname))
	h.Write([]byte(username))
	return h.Sum(nil), nil
}

// encrypt encrypts plaintext using AES-256-GCM and returns base58-encoded ciphertext
func (s *credentialStorage) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base58.Encode(ciphertext), nil
}

// decrypt decrypts base58-encoded ciphertext using AES-256-GCM
func (s *credentialStorage) decrypt(ciphertext string) (string, error) {
	data := base58.Decode(ciphertext)
	if len(data) == 0 {
		return "", errors.New("failed to decode base58")
	}

	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// readCredentials reads all credentials from the TOML file
func (s *credentialStorage) readCredentials() (map[string]*Cred, error) {
	file, err := os.Open(s.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*Cred), nil
		}
		return nil, fmt.Errorf("failed to open credentials file: %w", err)
	}
	defer file.Close() // nolint

	var credFile credentialsFile
	if _, err := toml.NewDecoder(file).Decode(&credFile); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	credentials := make(map[string]*Cred, len(credFile.Credentials))
	for _, entry := range credFile.Credentials {
		cred, ok := s.decryptCredentialEntry(entry)
		if !ok {
			continue // Skip unparseable entries
		}
		credentials[cred.target] = cred.Cred
	}

	return credentials, nil
}

// decryptedCredential holds a decrypted credential with its target
type decryptedCredential struct {
	*Cred
	target string
}

// decryptCredentialEntry decrypts a credential entry
func (s *credentialStorage) decryptCredentialEntry(entry credentialEntry) (*decryptedCredential, bool) {
	target, err := s.decrypt(entry.Target)
	if err != nil {
		return nil, false
	}

	username, err := s.decrypt(entry.Username)
	if err != nil {
		return nil, false
	}

	password, err := s.decrypt(entry.Password)
	if err != nil {
		return nil, false
	}

	cred := parseTargetName(target)
	cred.UserName = username
	cred.Password = password

	return &decryptedCredential{Cred: cred, target: target}, true
}

// writeCredentials writes all credentials to the TOML file
func (s *credentialStorage) writeCredentials(credentials map[string]*Cred) error {
	credFile := credentialsFile{
		Credentials: make([]credentialEntry, 0, len(credentials)),
	}

	// Use maps.Keys for deterministic iteration (Go 1.23+)
	// Build entries in sorted order for reproducible output
	keys := slices.Sorted(maps.Keys(credentials))
	for _, target := range keys {
		cred := credentials[target]
		entry, err := s.encryptCredentialEntry(target, cred)
		if err != nil {
			return err
		}
		credFile.Credentials = append(credFile.Credentials, entry)
	}

	file, err := os.OpenFile(s.storagePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create credentials file: %w", err)
	}
	defer file.Close() // nolint

	if err := toml.NewEncoder(file).Encode(credFile); err != nil {
		return fmt.Errorf("failed to encode credentials to TOML: %w", err)
	}

	return nil
}

// encryptCredentialEntry encrypts a credential entry
func (s *credentialStorage) encryptCredentialEntry(target string, cred *Cred) (credentialEntry, error) {
	encryptedTarget, err := s.encrypt(target)
	if err != nil {
		return credentialEntry{}, fmt.Errorf("failed to encrypt target: %w", err)
	}

	encryptedUsername, err := s.encrypt(cred.UserName)
	if err != nil {
		return credentialEntry{}, fmt.Errorf("failed to encrypt username: %w", err)
	}

	encryptedPassword, err := s.encrypt(cred.Password)
	if err != nil {
		return credentialEntry{}, fmt.Errorf("failed to encrypt password: %w", err)
	}

	return credentialEntry{
		Target:   encryptedTarget,
		Username: encryptedUsername,
		Password: encryptedPassword,
	}, nil
}

// Get retrieves credentials from the file storage
func (s *credentialStorage) Get(ctx context.Context, cred *Cred) (*Cred, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	credentials, err := s.readCredentials()
	if err != nil {
		return nil, err
	}

	target := buildTargetName(cred)
	stored, ok := credentials[target]
	if !ok {
		return nil, ErrNotFound
	}

	return stored, nil
}

// Store saves credentials to the file storage
func (s *credentialStorage) Store(ctx context.Context, cred *Cred) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if cred == nil || cred.UserName == "" || cred.Password == "" {
		return errors.New("invalid credential")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	credentials, err := s.readCredentials()
	if err != nil {
		return err
	}

	credentials[buildTargetName(cred)] = cred
	return s.writeCredentials(credentials)
}

// Erase removes credentials from the file storage
func (s *credentialStorage) Erase(ctx context.Context, cred *Cred) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	credentials, err := s.readCredentials()
	if err != nil {
		return err
	}

	target := buildTargetName(cred)
	if _, ok := credentials[target]; !ok {
		return ErrNotFound
	}

	delete(credentials, target)
	return s.writeCredentials(credentials)
}

// Name returns the storage name
func (s *credentialStorage) Name() string {
	return "file"
}
