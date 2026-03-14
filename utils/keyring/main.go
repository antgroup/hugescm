// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

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
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/base58"
)

// Cred represents a credential
type Cred struct {
	Protocol string
	Server   string
	Port     int
	Path     string
	UserName string
	Password string
}

// credentialStorage implements encrypted file-based credential storage
type credentialStorage struct {
	mu          sync.Mutex
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

const nonceSize = 12

func main() {
	fmt.Println("=== Keyring File Storage Test ===")

	// Create a temporary test file
	tmpFile := "/tmp/zeta-credentials-test-" + time.Now().Format("20060102-150405") + ".toml"
	defer func() { _ = os.Remove(tmpFile) }()

	// Test 1: Create storage with auto-derived key
	fmt.Println("Test 1: Create storage with auto-derived key")
	storage1, err := newCredentialStorage("", tmpFile)
	if err != nil {
		fmt.Printf("❌ Failed to create storage: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Storage created successfully\n")
	fmt.Printf("   Storage path: %s\n\n", tmpFile)

	// Test 2: Store credentials
	fmt.Println("Test 2: Store credentials")
	cred1 := &Cred{
		Protocol: "https",
		Server:   "code.alipay.com",
		Port:     443,
		Path:     "/zeta/zeta.git",
		UserName: "test-user",
		Password: "test-password-123",
	}

	ctx := context.Background()
	if err := storage1.Store(ctx, cred1); err != nil {
		fmt.Printf("❌ Failed to store credentials: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Credentials stored successfully\n")
	fmt.Printf("   Server: %s\n", cred1.Server)
	fmt.Printf("   User: %s\n\n", cred1.UserName)

	// Test 3: Retrieve credentials
	fmt.Println("Test 3: Retrieve credentials")
	retrieved, err := storage1.Get(ctx, &Cred{
		Protocol: "https",
		Server:   "code.alipay.com",
		Port:     443,
		Path:     "/zeta/zeta.git",
	})
	if err != nil {
		fmt.Printf("❌ Failed to retrieve credentials: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Credentials retrieved successfully\n")
	fmt.Printf("   Username: %s\n", retrieved.UserName)
	fmt.Printf("   Password: %s\n\n", retrieved.Password)

	// Verify
	if retrieved.UserName != cred1.UserName || retrieved.Password != cred1.Password {
		fmt.Printf("❌ Credentials mismatch!\n")
		os.Exit(1)
	}

	// Test 4: Store multiple credentials
	fmt.Println("Test 4: Store multiple credentials")
	cred2 := &Cred{
		Protocol: "https",
		Server:   "github.com",
		UserName: "github-user",
		Password: "github-token-xyz",
	}
	cred3 := &Cred{
		Protocol: "ssh",
		Server:   "gitlab.com",
		Port:     22,
		UserName: "gitlab-user",
		Password: "gitlab-key-abc",
	}

	if err := storage1.Store(ctx, cred2); err != nil {
		fmt.Printf("❌ Failed to store cred2: %v\n", err)
		os.Exit(1)
	}
	if err := storage1.Store(ctx, cred3); err != nil {
		fmt.Printf("❌ Failed to store cred3: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Multiple credentials stored successfully\n\n")

	// Test 5: Retrieve all credentials
	fmt.Println("Test 5: Retrieve all credentials")
	allCreds := []*Cred{
		{Protocol: "https", Server: "code.alipay.com", Port: 443, Path: "/zeta/zeta.git"},
		{Protocol: "https", Server: "github.com"},
		{Protocol: "ssh", Server: "gitlab.com", Port: 22},
	}

	for _, c := range allCreds {
		retrieved, err := storage1.Get(ctx, c)
		if err != nil {
			fmt.Printf("❌ Failed to retrieve credentials for %s: %v\n", c.Server, err)
			os.Exit(1)
		}
		fmt.Printf("   ✅ %s - %s\n", c.Server, retrieved.UserName)
	}
	fmt.Println()

	// Test 6: Update credentials
	fmt.Println("Test 6: Update credentials")
	cred1Updated := &Cred{
		Protocol: "https",
		Server:   "code.alipay.com",
		Port:     443,
		Path:     "/zeta/zeta.git",
		UserName: "updated-user",
		Password: "updated-password-456",
	}
	if err := storage1.Store(ctx, cred1Updated); err != nil {
		fmt.Printf("❌ Failed to update credentials: %v\n", err)
		os.Exit(1)
	}

	retrieved, err = storage1.Get(ctx, &Cred{
		Protocol: "https",
		Server:   "code.alipay.com",
		Port:     443,
		Path:     "/zeta/zeta.git",
	})
	if err != nil {
		fmt.Printf("❌ Failed to retrieve updated credentials: %v\n", err)
		os.Exit(1)
	}
	if retrieved.UserName != cred1Updated.UserName || retrieved.Password != cred1Updated.Password {
		fmt.Printf("❌ Updated credentials mismatch!\n")
		os.Exit(1)
	}
	fmt.Printf("✅ Credentials updated successfully\n\n")

	// Test 7: Erase credentials
	fmt.Println("Test 7: Erase credentials")
	if err := storage1.Erase(ctx, &Cred{
		Protocol: "https",
		Server:   "github.com",
	}); err != nil {
		fmt.Printf("❌ Failed to erase credentials: %v\n", err)
		os.Exit(1)
	}

	_, err = storage1.Get(ctx, &Cred{
		Protocol: "https",
		Server:   "github.com",
	})
	if err != ErrNotFound {
		fmt.Printf("❌ Expected ErrNotFound, got: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Credentials erased successfully\n\n")

	// Test 8: Show TOML file content
	fmt.Println("Test 8: TOML file content (encrypted)")
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		fmt.Printf("❌ Failed to read TOML file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", string(data))

	// Test 9: Test with custom encryption key
	fmt.Println("Test 9: Test with custom encryption key")
	customKey := "my-secret-key-12345"
	storage2, err := newCredentialStorage(customKey, tmpFile+".2")
	if err != nil {
		fmt.Printf("❌ Failed to create storage with custom key: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.Remove(tmpFile + ".2") }()

	if err := storage2.Store(ctx, &Cred{
		Protocol: "https",
		Server:   "example.com",
		UserName: "user",
		Password: "pass",
	}); err != nil {
		fmt.Printf("❌ Failed to store with custom key: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Custom encryption key works\n\n")

	// Test 10: Test with base58-encoded key
	fmt.Println("Test 10: Test with base58-encoded key")
	// Generate a valid 32-byte key and encode it
	base58Key := base58.Encode([]byte("12345678901234567890123456789012")) // 32 bytes
	storage3, err := newCredentialStorage(base58Key, tmpFile+".3")
	if err != nil {
		fmt.Printf("❌ Failed to create storage with base58 key: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.Remove(tmpFile + ".3") }()

	if err := storage3.Store(ctx, &Cred{
		Protocol: "https",
		Server:   "example2.com",
		UserName: "user2",
		Password: "pass2",
	}); err != nil {
		fmt.Printf("❌ Failed to store with base58 key: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Base58-encoded encryption key works\n\n")

	fmt.Println("=== All Tests Passed! ===")
}

var ErrNotFound = errors.New("credential not found")

// newCredentialStorage creates a new file-based credential storage
func newCredentialStorage(encryptionKey, storagePath string) (*credentialStorage, error) {
	key, err := deriveOrValidateKey(encryptionKey)
	if err != nil {
		return nil, err
	}

	return &credentialStorage{
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
		// Pad to 32 bytes if needed
		if len(keyBytes) < 32 {
			padded := make([]byte, 32)
			copy(padded, keyBytes)
			return padded, nil
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

// deriveEncryptionKey derives an AES-256 key from system-specific information
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
	defer func() { _ = file.Close() }()

	var credFile credentialsFile
	if _, err := toml.NewDecoder(file).Decode(&credFile); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	credentials := make(map[string]*Cred, len(credFile.Credentials))
	for _, entry := range credFile.Credentials {
		cred, ok := s.decryptCredentialEntry(entry)
		if !ok {
			continue
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
	keys := slices.Sorted(maps.Keys(credentials))
	for _, target := range keys {
		cred := credentials[target]
		entry, err := s.encryptCredentialEntry(target, cred)
		if err != nil {
			return err
		}
		credFile.Credentials = append(credFile.Credentials, entry)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(s.storagePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.OpenFile(s.storagePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create credentials file: %w", err)
	}
	defer func() { _ = file.Close() }()

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

// buildTargetName creates a unique target name for storing credentials
// Format: "zeta+<protocol>://<server>[:<port>][<path>]"
func buildTargetName(cred *Cred) string {
	protocol := cred.Protocol
	if protocol == "" {
		protocol = "https"
	}

	var host string
	if cred.Port != 0 {
		host = net.JoinHostPort(cred.Server, strconv.Itoa(cred.Port))
	} else {
		host = cred.Server
	}

	u := &url.URL{
		Scheme: "zeta+" + protocol,
		Host:   host,
		Path:   cred.Path,
	}
	return u.String()
}

// parseTargetName parses a target name back into a Cred struct
// Format: "zeta+<protocol>://<server>[:<port>][<path>]"
func parseTargetName(target string) *Cred {
	u, err := url.Parse(target)
	if err != nil {
		return &Cred{Server: target}
	}

	// Extract protocol from "zeta+<protocol>" scheme
	scheme := u.Scheme
	protocol, found := parseSchemePrefix(scheme, "zeta+")
	if !found {
		return &Cred{Server: target}
	}

	cred := &Cred{
		Protocol: protocol,
		Server:   u.Hostname(),
		Path:     u.Path,
	}

	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			cred.Port = port
		}
	}
	return cred
}

// parseSchemePrefix parses a scheme like "zeta+https" and returns the protocol part
func parseSchemePrefix(scheme, prefix string) (protocol string, found bool) {
	if len(scheme) <= len(prefix) {
		return "", false
	}
	if scheme[:len(prefix)] != prefix {
		return "", false
	}
	return scheme[len(prefix):], true
}
