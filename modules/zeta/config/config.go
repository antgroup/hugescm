// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"

	"github.com/antgroup/hugescm/modules/strengthen"
)

const (
	FragmentThreshold int64 = 1 * strengthen.GiByte // 1G
	FragmentSize      int64 = 1 * strengthen.GiByte // 1G
)

// ErrBadConfigKey indicates an invalid configuration key was provided.
type ErrBadConfigKey struct {
	key string
}

func (err *ErrBadConfigKey) Error() string {
	return fmt.Sprintf("bad zeta config key '%s'", err.key)
}

func IsErrBadConfigKey(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrBadConfigKey)
	return ok
}

var (
	ErrInvalidArgument = errors.New("invalid argument")
)

type User struct {
	Name  string `toml:"name,omitempty"`
	Email string `toml:"email,omitempty"`
}

func (u *User) Empty() bool {
	return u == nil || len(u.Email) == 0 || len(u.Name) == 0
}

func overwrite(current, override string) string {
	if override != "" {
		return override
	}
	return current
}

func (u *User) Overwrite(o *User) {
	u.Name = overwrite(u.Name, o.Name)
	u.Email = overwrite(u.Email, o.Email)
}

type Core struct {
	SharingRoot         string      `toml:"sharingRoot,omitempty"` // GLOBAL
	HooksPath           string      `toml:"hooksPath,omitempty"`   // GLOBAL
	Remote              string      `toml:"remote,omitempty"`
	Snapshot            bool        `toml:"snapshot,omitempty"`
	SparseDirs          StringArray `toml:"sparse,omitempty"`
	HashALGO            string      `toml:"hash-algo,omitempty"`
	CompressionALGO     string      `toml:"compression-algo,omitempty"`
	Editor              string      `toml:"editor,omitempty"`
	OptimizeStrategy    Strategy    `toml:"optimizeStrategy,omitempty"`   // zeta config core.optimizeStrategy eager OR ZETA_CORE_OPTIMIZE_STRATEGY="eager"
	Accelerator         Accelerator `toml:"accelerator,omitempty"`        // zeta config core.accelerator dragonfly OR ZETA_CORE_ACCELERATOR="dragonfly"
	ConcurrentTransfers int         `toml:"concurrenttransfers,omitzero"` // zeta config core.concurrenttransfers 8 OR ZETA_CORE_CONCURRENT_TRANSFERS=8
}

func (c *Core) Overwrite(o *Core) {
	c.SharingRoot = overwrite(c.SharingRoot, o.SharingRoot)
	c.HooksPath = overwrite(c.HooksPath, o.HooksPath)
	c.Remote = overwrite(c.Remote, o.Remote)
	c.Snapshot = o.Snapshot
	if len(o.Accelerator) != 0 {
		c.Accelerator = o.Accelerator
	}
	if len(o.OptimizeStrategy) != 0 {
		c.OptimizeStrategy = o.OptimizeStrategy
	}
	if o.ConcurrentTransfers > 0 {
		c.ConcurrentTransfers = o.ConcurrentTransfers
	}
	c.CompressionALGO = overwrite(c.CompressionALGO, o.CompressionALGO)
	c.Editor = overwrite(c.Editor, o.Editor)
	// merge sparse dirs
	if len(o.SparseDirs) != 0 {
		c.SparseDirs = o.SparseDirs
	}
}

// IsExtreme: Extreme cleanup strategy to delete large object snapshots in the repository. Typically used in AI scenarios, it is no longer necessary to save blobs when downloading models.
func (c *Core) IsExtreme() bool {
	return c.OptimizeStrategy == StrategyExtreme
}

type Fragment struct {
	ThresholdRaw Size    `toml:"threshold,omitempty"`
	SizeRaw      Size    `toml:"size,omitempty"`
	EnableCDC    Boolean `toml:"enable_cdc,omitempty"` // Enable CDC (Content-Defined Chunking) for AI model files
}

func (f *Fragment) Overwrite(o *Fragment) {
	if o.ThresholdRaw > 0 {
		f.ThresholdRaw = o.ThresholdRaw
	}
	if o.SizeRaw > 0 {
		f.SizeRaw = o.SizeRaw
	}
	f.EnableCDC.Merge(&o.EnableCDC)
}

func (f Fragment) Threshold() int64 {
	if f.ThresholdRaw < strengthen.MiByte {
		return FragmentThreshold
	}
	return int64(f.ThresholdRaw)
}

func (f Fragment) Size() int64 {
	if f.SizeRaw < strengthen.MiByte {
		return FragmentSize
	}
	return int64(f.SizeRaw)
}

type HTTP struct {
	ExtraHeader StringArray `toml:"extraHeader,omitempty"`
	SSLVerify   Boolean     `toml:"sslVerify,omitempty"`
}

func (h *HTTP) Overwrite(o *HTTP) {
	if len(o.ExtraHeader) > 0 {
		h.ExtraHeader = append(h.ExtraHeader, o.ExtraHeader...)
	}
	h.SSLVerify.Merge(&o.SSLVerify)
}

type SSH struct {
	ExtraEnv StringArray `toml:"extraEnv,omitempty"`
}

func (u *SSH) Overwrite(o *SSH) {
	if len(o.ExtraEnv) > 0 {
		u.ExtraEnv = append(u.ExtraEnv, o.ExtraEnv...)
	}
}

type Transport struct {
	MaxEntries    int    `toml:"maxEntries,omitempty"`
	LargeSizeRaw  Size   `toml:"largeSize,omitempty"`
	ExternalProxy string `toml:"externalProxy,omitempty"`
}

const (
	minLargeSize = 512 << 10 // 512K
	largeSize    = 5 << 20   // 5M
)

func (t Transport) LargeSize() int64 {
	if t.LargeSizeRaw < minLargeSize {
		return largeSize
	}
	return int64(t.LargeSizeRaw)
}

func (t *Transport) Overwrite(o *Transport) {
	if o.LargeSizeRaw >= minLargeSize {
		t.LargeSizeRaw = o.LargeSizeRaw
	}
	if o.MaxEntries > 0 {
		t.MaxEntries = o.MaxEntries
	}
	t.ExternalProxy = overwrite(t.ExternalProxy, o.ExternalProxy)
}

type Diff struct {
	Algorithm string `toml:"algorithm,omitempty"`
}

func (d *Diff) Overwrite(o *Diff) {
	d.Algorithm = overwrite(d.Algorithm, o.Algorithm)
}

type Merge struct {
	ConflictStyle string `toml:"conflictStyle,omitempty"`
}

func (m *Merge) Overwrite(o *Merge) {
	m.ConflictStyle = overwrite(m.ConflictStyle, o.ConflictStyle)
}

// Credential configures credential storage behavior.
// Different platforms support different storage backends:
//
// macOS:
//   - Default: Uses Security.framework via purego (no CGO required)
//   - "security": Uses /usr/bin/security CLI tool (fallback when security software blocks framework access)
//   - "file": Uses encrypted file storage
//
// Windows:
//   - Default: Uses Windows Credential Manager API
//   - "file": Uses encrypted file storage
//
// Linux:
//   - Default: "none" (credentials not stored unless explicitly configured)
//   - "secret-service": Uses libsecret/Secret Service API (requires DBUS)
//   - "file": Uses encrypted file storage
type Credential struct {
	// Storage specifies the credential storage backend.
	//
	// Common options:
	//   - "auto" (default): Use the platform's default backend
	//   - "file": Use encrypted file storage (requires encryptionKey)
	//   - "none": Disable credential storage completely
	//
	// Platform-specific options:
	//   - macOS: "security" (uses /usr/bin/security CLI)
	//   - Linux: "secret-service" (requires DBUS/Secret Service)
	//
	// Can be set via: zeta config credential.storage <value>
	// Or environment: ZETA_CREDENTIAL_STORAGE=<value>
	Storage string `toml:"storage,omitempty"`

	// EncryptionKey specifies the key used for encrypting credentials in file storage.
	// Required when storage="file". If not set, falls back to "auto" mode.
	//
	// Security note: Store this key securely! Consider using environment variable:
	//   ZETA_CREDENTIAL_ENCRYPTION_KEY=<key>
	//
	// To generate a secure key: openssl rand -base64 32
	EncryptionKey string `toml:"encryptionKey,omitempty"`

	// StoragePath specifies the path for encrypted credential file storage.
	// Only used when storage="file".
	// Default: ~/.config/zeta/credentials
	//
	// Can be set via: zeta config credential.storagePath <path>
	// Or environment: ZETA_CREDENTIAL_STORAGE_PATH=<path>
	StoragePath string `toml:"storagePath,omitempty"`
}

// CredentialStorageConstants defines valid storage backend values
const (
	CredentialStorageAuto          = "auto"           // Default backend for each platform
	CredentialStorageSecretService = "secret-service" // Linux: Secret Service API (libsecret)
	CredentialStorageFile          = "file"           // All platforms: encrypted file storage
	CredentialStorageNone          = "none"           // Disable credential storage
	CredentialStorageSecurity      = "security"       // macOS: /usr/bin/security CLI
)

func (c *Credential) Overwrite(o *Credential) {
	c.Storage = overwrite(c.Storage, o.Storage)
	c.EncryptionKey = overwrite(c.EncryptionKey, o.EncryptionKey)
	c.StoragePath = overwrite(c.StoragePath, o.StoragePath)
}

type Config struct {
	Core       Core       `toml:"core,omitempty"`
	User       User       `toml:"user,omitempty"`
	Fragment   Fragment   `toml:"fragment,omitempty"`
	HTTP       HTTP       `toml:"http,omitempty"`
	SSH        SSH        `toml:"ssh,omitempty"`
	Transport  Transport  `toml:"transport,omitempty"`
	Diff       Diff       `toml:"diff,omitempty"`
	Merge      Merge      `toml:"merge,omitempty"`
	Credential Credential `toml:"credential,omitempty"`
}

// Overwrite: use local config overwrite config
func (c *Config) Overwrite(other *Config) {
	c.Core.Overwrite(&other.Core)
	c.User.Overwrite(&other.User)
	c.Fragment.Overwrite(&other.Fragment)
	c.HTTP.Overwrite(&other.HTTP)
	c.SSH.Overwrite(&other.SSH)
	c.Transport.Overwrite(&other.Transport)
	c.Diff.Overwrite(&other.Diff)
	c.Merge.Overwrite(&other.Merge)
	c.Credential.Overwrite(&other.Credential)
}
