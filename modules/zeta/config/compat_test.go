// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

// TestCompatConfigDecoding tests that the new implementation decodes Config
// structs with the same semantics as the old implementation.
func TestCompatConfigDecoding(t *testing.T) {
	tests := []struct {
		name string
		toml string
		want Config
	}{
		{
			name: "basic config",
			toml: `
[core]
editor = "vim"
remote = "origin"
snapshot = true

[user]
name = "Alice"
email = "alice@example.com"
`,
			want: Config{
				Core: Core{
					Editor:   "vim",
					Remote:   "origin",
					Snapshot: true,
				},
				User: User{
					Name:  "Alice",
					Email: "alice@example.com",
				},
			},
		},
		{
			name: "with size values",
			toml: `
[fragment]
threshold = "2g"
size = "1g"

[transport]
largeSize = "10m"
maxEntries = 8
`,
			want: Config{
				Fragment: Fragment{
					ThresholdRaw: 2 * 1024 * 1024 * 1024,
					SizeRaw:      1 * 1024 * 1024 * 1024,
				},
				Transport: Transport{
					LargeSizeRaw: 10 * 1024 * 1024,
					MaxEntries:   8,
				},
			},
		},
		{
			name: "with string array",
			toml: `
[core]
sparse = ["dir1", "dir2", "dir3"]

[http]
extraHeader = ["X-Custom: value1", "X-Custom: value2"]
`,
			want: Config{
				Core: Core{
					SparseDirs: []string{"dir1", "dir2", "dir3"},
				},
				HTTP: HTTP{
					ExtraHeader: []string{"X-Custom: value1", "X-Custom: value2"},
				},
			},
		},
		{
			name: "with boolean",
			toml: `
[http]
sslVerify = true

[fragment]
enable_cdc = false
`,
			want: Config{
				HTTP: HTTP{
					SSLVerify: True,
				},
				Fragment: Fragment{
					EnableCDC: False,
				},
			},
		},
		{
			name: "with credential",
			toml: `
[credential]
storage = "file"
encryptionKey = "secret-key"
storagePath = "/path/to/creds"
`,
			want: Config{
				Credential: Credential{
					Storage:       "file",
					EncryptionKey: "secret-key",
					StoragePath:   "/path/to/creds",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			err := LoadConfig([]byte(tt.toml), &cfg)
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}

			// Compare Core
			if cfg.Core.Editor != tt.want.Core.Editor {
				t.Errorf("Core.Editor = %q, want %q", cfg.Core.Editor, tt.want.Core.Editor)
			}
			if cfg.Core.Remote != tt.want.Core.Remote {
				t.Errorf("Core.Remote = %q, want %q", cfg.Core.Remote, tt.want.Core.Remote)
			}
			if cfg.Core.Snapshot != tt.want.Core.Snapshot {
				t.Errorf("Core.Snapshot = %v, want %v", cfg.Core.Snapshot, tt.want.Core.Snapshot)
			}
			if !stringSlicesEqual(cfg.Core.SparseDirs, tt.want.Core.SparseDirs) {
				t.Errorf("Core.SparseDirs = %v, want %v", cfg.Core.SparseDirs, tt.want.Core.SparseDirs)
			}

			// Compare User
			if cfg.User.Name != tt.want.User.Name {
				t.Errorf("User.Name = %q, want %q", cfg.User.Name, tt.want.User.Name)
			}
			if cfg.User.Email != tt.want.User.Email {
				t.Errorf("User.Email = %q, want %q", cfg.User.Email, tt.want.User.Email)
			}

			// Compare Fragment
			if cfg.Fragment.ThresholdRaw != tt.want.Fragment.ThresholdRaw {
				t.Errorf("Fragment.ThresholdRaw = %d, want %d", cfg.Fragment.ThresholdRaw, tt.want.Fragment.ThresholdRaw)
			}
			if cfg.Fragment.SizeRaw != tt.want.Fragment.SizeRaw {
				t.Errorf("Fragment.SizeRaw = %d, want %d", cfg.Fragment.SizeRaw, tt.want.Fragment.SizeRaw)
			}
			if cfg.Fragment.EnableCDC.True() != tt.want.Fragment.EnableCDC.True() {
				t.Errorf("Fragment.EnableCDC = %v, want %v", cfg.Fragment.EnableCDC.True(), tt.want.Fragment.EnableCDC.True())
			}

			// Compare HTTP
			if !stringSlicesEqual(cfg.HTTP.ExtraHeader, tt.want.HTTP.ExtraHeader) {
				t.Errorf("HTTP.ExtraHeader = %v, want %v", cfg.HTTP.ExtraHeader, tt.want.HTTP.ExtraHeader)
			}
			if cfg.HTTP.SSLVerify.True() != tt.want.HTTP.SSLVerify.True() {
				t.Errorf("HTTP.SSLVerify = %v, want %v", cfg.HTTP.SSLVerify.True(), tt.want.HTTP.SSLVerify.True())
			}

			// Compare Transport
			if cfg.Transport.LargeSizeRaw != tt.want.Transport.LargeSizeRaw {
				t.Errorf("Transport.LargeSizeRaw = %d, want %d", cfg.Transport.LargeSizeRaw, tt.want.Transport.LargeSizeRaw)
			}
			if cfg.Transport.MaxEntries != tt.want.Transport.MaxEntries {
				t.Errorf("Transport.MaxEntries = %d, want %d", cfg.Transport.MaxEntries, tt.want.Transport.MaxEntries)
			}

			// Compare Credential
			if cfg.Credential.Storage != tt.want.Credential.Storage {
				t.Errorf("Credential.Storage = %q, want %q", cfg.Credential.Storage, tt.want.Credential.Storage)
			}
			if cfg.Credential.EncryptionKey != tt.want.Credential.EncryptionKey {
				t.Errorf("Credential.EncryptionKey = %q, want %q", cfg.Credential.EncryptionKey, tt.want.Credential.EncryptionKey)
			}
			if cfg.Credential.StoragePath != tt.want.Credential.StoragePath {
				t.Errorf("Credential.StoragePath = %q, want %q", cfg.Credential.StoragePath, tt.want.Credential.StoragePath)
			}
		})
	}
}

// TestCompatOverwrite tests that Overwrite methods maintain the same semantics.
func TestCompatOverwrite(t *testing.T) {
	t.Run("Core.Overwrite", func(t *testing.T) {
		base := Core{
			Editor:   "vim",
			Remote:   "origin",
			Snapshot: false,
		}
		override := Core{
			Editor:   "nano",
			Remote:   "", // Empty string should not override
			Snapshot: true,
		}
		base.Overwrite(&override)

		if base.Editor != "nano" {
			t.Errorf("Editor = %q, want nano", base.Editor)
		}
		if base.Remote != "origin" {
			t.Errorf("Remote = %q, want origin (not overwritten)", base.Remote)
		}
		if !base.Snapshot {
			t.Errorf("Snapshot = false, want true")
		}
	})

	t.Run("User.Overwrite", func(t *testing.T) {
		base := User{
			Name:  "Alice",
			Email: "alice@example.com",
		}
		override := User{
			Name:  "Bob",
			Email: "", // Empty should not override
		}
		base.Overwrite(&override)

		if base.Name != "Bob" {
			t.Errorf("Name = %q, want Bob", base.Name)
		}
		if base.Email != "alice@example.com" {
			t.Errorf("Email = %q, want alice@example.com (not overwritten)", base.Email)
		}
	})

	t.Run("HTTP.Overwrite merges ExtraHeader", func(t *testing.T) {
		base := HTTP{
			ExtraHeader: []string{"X-Header: value1"},
		}
		override := HTTP{
			ExtraHeader: []string{"X-Header: value2"},
		}
		base.Overwrite(&override)

		if len(base.ExtraHeader) != 2 {
			t.Errorf("ExtraHeader len = %d, want 2", len(base.ExtraHeader))
		}
	})

	t.Run("Config.Overwrite priority", func(t *testing.T) {
		base := Config{
			Core: Core{
				Editor: "vim",
			},
			User: User{
				Name: "Alice",
			},
		}
		override := Config{
			Core: Core{
				Editor: "nano",
			},
			User: User{
				Name: "Bob",
			},
		}
		base.Overwrite(&override)

		if base.Core.Editor != "nano" {
			t.Errorf("Core.Editor = %q, want nano", base.Core.Editor)
		}
		if base.User.Name != "Bob" {
			t.Errorf("User.Name = %q, want Bob", base.User.Name)
		}
	})
}

// TestCompatBooleanMerge tests Boolean.Merge semantics.
func TestCompatBooleanMerge(t *testing.T) {
	tests := []struct {
		name     string
		base     Boolean
		other    Boolean
		expected int
	}{
		{"UNSET + TRUE = TRUE", Boolean{val: BOOLEAN_UNSET}, Boolean{val: BOOLEAN_TRUE}, BOOLEAN_TRUE},
		{"UNSET + FALSE = FALSE", Boolean{val: BOOLEAN_UNSET}, Boolean{val: BOOLEAN_FALSE}, BOOLEAN_FALSE},
		{"TRUE + FALSE = FALSE (higher priority)", Boolean{val: BOOLEAN_TRUE}, Boolean{val: BOOLEAN_FALSE}, BOOLEAN_FALSE},
		{"FALSE + TRUE = TRUE (higher priority)", Boolean{val: BOOLEAN_FALSE}, Boolean{val: BOOLEAN_TRUE}, BOOLEAN_TRUE},
		{"TRUE + UNSET = TRUE", Boolean{val: BOOLEAN_TRUE}, Boolean{val: BOOLEAN_UNSET}, BOOLEAN_TRUE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.base
			b.Merge(&tt.other)
			if b.val != tt.expected {
				t.Errorf("Merge() = %v, want %v", b.val, tt.expected)
			}
		})
	}
}

// TestCompatKeyParsing tests that key parsing maintains the same semantics.
func TestCompatKeyParsing(t *testing.T) {
	// Valid keys
	validKeys := []string{
		"core.editor",
		"http.sslVerify",
		"user.name",
		"transport.maxEntries",
	}

	for _, key := range validKeys {
		t.Run("valid: "+key, func(t *testing.T) {
			_, err := ParseKey(key)
			if err != nil {
				t.Errorf("ParseKey(%q) error: %v", key, err)
			}
		})
	}

	// Invalid keys
	invalidKeys := []string{
		"core",    // Missing dot
		".editor", // Missing section
		"core.",   // Missing name
		"a.b.c",   // Nested path
		"",        // Empty
	}

	for _, key := range invalidKeys {
		t.Run("invalid: "+key, func(t *testing.T) {
			_, err := ParseKey(key)
			if err == nil {
				t.Errorf("ParseKey(%q) expected error, got nil", key)
			}
		})
	}
}

// Helper function
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
