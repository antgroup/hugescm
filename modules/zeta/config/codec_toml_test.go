// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestLoadDocument(t *testing.T) {
	tomlData := `
[core]
editor = "vim"
sparse = ["dir1", "dir2", "dir3"]
timeout = 30

[user]
name = "Alice"
email = "alice@example.com"

[http]
sslVerify = true
maxRetries = 5
`

	doc, err := LoadDocument([]byte(tomlData))
	if err != nil {
		t.Fatalf("LoadDocument() error: %v", err)
	}

	// Test string value
	value, exists, err := doc.Get("core.editor")
	if err != nil {
		t.Fatalf("Get(core.editor) error: %v", err)
	}
	if !exists {
		t.Fatalf("Get(core.editor) not found")
	}
	if value.Kind() != KindString {
		t.Errorf("core.editor kind = %v, want %v", value.Kind(), KindString)
	}
	if value.ToAny() != "vim" {
		t.Errorf("core.editor = %v, want vim", value.ToAny())
	}

	// Test string slice
	value, exists, err = doc.Get("core.sparse")
	if err != nil {
		t.Fatalf("Get(core.sparse) error: %v", err)
	}
	if !exists {
		t.Fatalf("Get(core.sparse) not found")
	}
	if value.Kind() != KindStringSlice {
		t.Errorf("core.sparse kind = %v, want %v", value.Kind(), KindStringSlice)
	}
	all := value.All()
	if len(all) != 3 {
		t.Errorf("core.sparse len = %d, want 3", len(all))
	}

	// Test int64 value
	value, exists, err = doc.Get("core.timeout")
	if err != nil {
		t.Fatalf("Get(core.timeout) error: %v", err)
	}
	if !exists {
		t.Fatalf("Get(core.timeout) not found")
	}
	if value.Kind() != KindInt64 {
		t.Errorf("core.timeout kind = %v, want %v", value.Kind(), KindInt64)
	}

	// Test bool value
	value, exists, err = doc.Get("http.sslVerify")
	if err != nil {
		t.Fatalf("Get(http.sslVerify) error: %v", err)
	}
	if !exists {
		t.Fatalf("Get(http.sslVerify) not found")
	}
	if value.Kind() != KindBool {
		t.Errorf("http.sslVerify kind = %v, want %v", value.Kind(), KindBool)
	}
}

func TestMarshalDocument(t *testing.T) {
	doc := NewDocument()
	_, _ = doc.Set("core.editor", "vim")
	_, _ = doc.Set("core.sparse", []string{"dir1", "dir2"})
	_, _ = doc.Set("user.name", "Bob")
	_, _ = doc.Set("http.timeout", int64(60))

	data, err := MarshalDocument(doc)
	if err != nil {
		t.Fatalf("MarshalDocument() error: %v", err)
	}

	// Parse it back
	doc2, err := LoadDocument(data)
	if err != nil {
		t.Fatalf("LoadDocument() error: %v", err)
	}

	// Verify round-trip
	value, exists, _ := doc2.Get("core.editor")
	if !exists || value.ToAny() != "vim" {
		t.Errorf("Round-trip core.editor failed")
	}

	value, exists, _ = doc2.Get("core.sparse")
	if !exists || value.Kind() != KindStringSlice {
		t.Errorf("Round-trip core.sparse failed")
	}

	value, exists, _ = doc2.Get("http.timeout")
	if !exists || value.Kind() != KindInt64 {
		t.Errorf("Round-trip http.timeout failed")
	}
}

func TestLoadConfig(t *testing.T) {
	tomlData := `
[core]
editor = "vim"
remote = "origin"
snapshot = true

[user]
name = "Charlie"
email = "charlie@example.com"

[fragment]
threshold = "2g"
size = "1g"

[http]
sslVerify = false
`

	var cfg Config
	err := LoadConfig([]byte(tomlData), &cfg)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	// Verify parsed config
	if cfg.Core.Editor != "vim" {
		t.Errorf("Core.Editor = %v, want vim", cfg.Core.Editor)
	}
	if cfg.User.Name != "Charlie" {
		t.Errorf("User.Name = %v, want Charlie", cfg.User.Name)
	}
	if cfg.User.Email != "charlie@example.com" {
		t.Errorf("User.Email = %v, want charlie@example.com", cfg.User.Email)
	}
	if !cfg.Core.Snapshot {
		t.Errorf("Core.Snapshot = false, want true")
	}
	if !cfg.HTTP.SSLVerify.False() {
		t.Errorf("HTTP.SSLVerify = true, want false")
	}
}

func TestValidateDocumentAs(t *testing.T) {
	// Valid document
	doc := NewDocument()
	_, _ = doc.Set("core.editor", "vim")
	_, _ = doc.Set("user.name", "Alice")

	var cfg Config
	err := ValidateDocumentAs(doc, &cfg)
	if err != nil {
		t.Errorf("ValidateDocumentAs() valid document error: %v", err)
	}
}

func TestLoadDocumentInvalidStructure(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr bool
	}{
		{
			name: "valid simple",
			toml: `
[core]
editor = "vim"
`,
			wantErr: false,
		},
		{
			name:    "top-level scalar key invalid for document model",
			toml:    `editor = "vim"`,
			wantErr: true,
		},
		{
			name: "nested table",
			toml: `
[core]
[core.nested]
key = "value"
`,
			wantErr: true,
		},
		{
			name: "array of tables not supported",
			toml: `
[[core.items]]
name = "item1"
`,
			wantErr: true,
		},
		{
			name: "empty array cannot infer type",
			toml: `
[core]
items = []
`,
			wantErr: true,
		},
		{
			name: "valid array",
			toml: `
[core]
items = ["a", "b"]
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadDocument([]byte(tt.toml))
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadDocument() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("LoadDocument() unexpected error: %v", err)
				}
			}
		})
	}
}
