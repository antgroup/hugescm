// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestParseKey(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantSection string
		wantName    string
		wantError   bool
	}{
		{
			name:        "valid key",
			input:       "core.editor",
			wantSection: "core",
			wantName:    "editor",
		},
		{
			name:        "valid key with hyphen",
			input:       "http.ssl-verify",
			wantSection: "http",
			wantName:    "ssl-verify",
		},
		{
			name:      "missing dot - core",
			input:     "core",
			wantError: true,
		},
		{
			name:      "missing name - core.",
			input:     "core.",
			wantError: true,
		},
		{
			name:      "missing section - .editor",
			input:     ".editor",
			wantError: true,
		},
		{
			name:      "nested path - a.b.c",
			input:     "a.b.c",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := ParseKey(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseKey(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseKey(%q) unexpected error: %v", tt.input, err)
				return
			}
			if key.Section != tt.wantSection {
				t.Errorf("ParseKey(%q) section = %q, want %q", tt.input, key.Section, tt.wantSection)
			}
			if key.Name != tt.wantName {
				t.Errorf("ParseKey(%q) name = %q, want %q", tt.input, key.Name, tt.wantName)
			}
		})
	}
}

func TestDocumentSetGet(t *testing.T) {
	doc := NewDocument()

	// Test Set and Get
	overwritten, err := doc.Set("core.editor", "vim")
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if overwritten {
		t.Errorf("Set() overwritten = true, want false (first set)")
	}

	value, exists, err := doc.Get("core.editor")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if !exists {
		t.Errorf("Get() exists = false, want true")
	}
	if value.Kind() != KindString {
		t.Errorf("Get() kind = %v, want %v", value.Kind(), KindString)
	}
	if value.ToAny() != "vim" {
		t.Errorf("Get() value = %v, want vim", value.ToAny())
	}

	// Test overwrite
	overwritten, err = doc.Set("core.editor", "nano")
	if err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if !overwritten {
		t.Errorf("Set() overwritten = false, want true (overwrite)")
	}

	value, _, _ = doc.Get("core.editor")
	if value.ToAny() != "nano" {
		t.Errorf("Get() value = %v, want nano", value.ToAny())
	}

	// Test GetFirst
	first, err := doc.GetFirst("core.editor")
	if err != nil {
		t.Fatalf("GetFirst() error: %v", err)
	}
	if first != "nano" {
		t.Errorf("GetFirst() = %v, want nano", first)
	}

	// Test non-existent key
	_, exists, err = doc.Get("nonexistent.key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if exists {
		t.Errorf("Get() exists = true for non-existent key, want false")
	}

	_, err = doc.GetFirst("nonexistent.key")
	if err != ErrKeyNotFound {
		t.Errorf("GetFirst() error = %v, want ErrKeyNotFound", err)
	}
}

func TestDocumentAdd(t *testing.T) {
	doc := NewDocument()

	// Add to non-existent key -> creates single value
	err := doc.Add("core.sparse", "dir1")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	value, _, _ := doc.Get("core.sparse")
	if value.Kind() != KindString {
		t.Errorf("Add() kind = %v, want %v", value.Kind(), KindString)
	}

	// Add to existing scalar -> creates slice
	err = doc.Add("core.sparse", "dir2")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	value, _, _ = doc.Get("core.sparse")
	if value.Kind() != KindStringSlice {
		t.Errorf("Add() kind = %v, want %v", value.Kind(), KindStringSlice)
	}
	all := value.All()
	if len(all) != 2 {
		t.Errorf("Add() len = %d, want 2", len(all))
	}

	// Add to existing slice -> appends
	err = doc.Add("core.sparse", "dir3")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	value, _, _ = doc.Get("core.sparse")
	all = value.All()
	if len(all) != 3 {
		t.Errorf("Add() len = %d, want 3", len(all))
	}

	// Type mismatch should error
	err = doc.Add("core.sparse", 123)
	if err == nil {
		t.Errorf("Add() expected error for type mismatch, got nil")
	}
}

func TestDocumentDelete(t *testing.T) {
	doc := NewDocument()

	// Set a value
	_, _ = doc.Set("core.editor", "vim")

	// Delete existing key
	err := doc.Delete("core.editor")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify deletion
	_, exists, _ := doc.Get("core.editor")
	if exists {
		t.Errorf("Get() exists = true after delete, want false")
	}

	// Delete non-existent key should return ErrKeyNotFound
	err = doc.Delete("nonexistent.key")
	if err != ErrKeyNotFound {
		t.Errorf("Delete() error = %v, want ErrKeyNotFound", err)
	}

	// Delete invalid key should return ErrBadConfigKey
	err = doc.Delete("invalid")
	if err == nil {
		t.Errorf("Delete() expected error for invalid key, got nil")
	}
}

func TestDocumentRawRoundTrip(t *testing.T) {
	doc := NewDocument()
	_, _ = doc.Set("core.editor", "vim")
	_, _ = doc.Set("core.sparse", []string{"dir1", "dir2"})
	_, _ = doc.Set("user.name", "Alice")
	_, _ = doc.Set("user.email", "alice@example.com")
	_, _ = doc.Set("http.timeout", int64(30))

	// Convert to raw
	raw := doc.Raw()

	// Verify raw structure
	if len(raw) != 3 {
		t.Errorf("Raw() len = %d, want 3", len(raw))
	}
	if raw["core"]["editor"] != "vim" {
		t.Errorf("Raw() core.editor = %v, want vim", raw["core"]["editor"])
	}

	// Convert back from raw
	doc2, err := FromRaw(raw)
	if err != nil {
		t.Fatalf("FromRaw() error: %v", err)
	}

	// Verify round-trip
	value, exists, _ := doc2.Get("core.editor")
	if !exists || value.ToAny() != "vim" {
		t.Errorf("Round-trip core.editor failed")
	}

	value, exists, _ = doc2.Get("user.name")
	if !exists || value.ToAny() != "Alice" {
		t.Errorf("Round-trip user.name failed")
	}
}

func TestDocumentGetAll(t *testing.T) {
	doc := NewDocument()

	// Single value
	_, _ = doc.Set("core.editor", "vim")
	all, err := doc.GetAll("core.editor")
	if err != nil {
		t.Fatalf("GetAll() error: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("GetAll() len = %d, want 1", len(all))
	}

	// Slice value
	_, _ = doc.Set("core.sparse", []string{"dir1", "dir2", "dir3"})
	all, err = doc.GetAll("core.sparse")
	if err != nil {
		t.Fatalf("GetAll() error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("GetAll() len = %d, want 3", len(all))
	}

	// Non-existent key
	_, err = doc.GetAll("nonexistent.key")
	if err != ErrKeyNotFound {
		t.Errorf("GetAll() error = %v, want ErrKeyNotFound", err)
	}
}

func TestDocumentBadKey(t *testing.T) {
	doc := NewDocument()

	// Test Get with bad key
	_, _, err := doc.Get("a.b.c")
	if err == nil {
		t.Errorf("Get(a.b.c) expected error, got nil")
	}
	if !IsErrBadConfigKey(err) {
		t.Errorf("Get(a.b.c) error = %v, want ErrBadConfigKey", err)
	}

	// Test Set with bad key
	_, err = doc.Set("a.b.c", "value")
	if err == nil {
		t.Errorf("Set(a.b.c) expected error, got nil")
	}
	if !IsErrBadConfigKey(err) {
		t.Errorf("Set(a.b.c) error = %v, want ErrBadConfigKey", err)
	}

	// Test Add with bad key
	err = doc.Add("a.b.c", "value")
	if err == nil {
		t.Errorf("Add(a.b.c) expected error, got nil")
	}
	if !IsErrBadConfigKey(err) {
		t.Errorf("Add(a.b.c) error = %v, want ErrBadConfigKey", err)
	}

	// Test Delete with bad key
	err = doc.Delete("a.b.c")
	if err == nil {
		t.Errorf("Delete(a.b.c) expected error, got nil")
	}
	if !IsErrBadConfigKey(err) {
		t.Errorf("Delete(a.b.c) error = %v, want ErrBadConfigKey", err)
	}
}
