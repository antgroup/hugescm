package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestEncode(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	doc, err := LoadDocumentFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		return
	}
	// Add user section
	_, _ = doc.Set("user.email", "zeta@example.io")
	_, _ = doc.Set("user.name", "bob")

	data, err := MarshalDocument(doc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(os.Stdout, "%s", data)
}

func TestUpdateConfig(t *testing.T) {
	values := map[string]any{
		"core.sharingRoot": "/tmp/sharingRoot",
		"user.email":       "zeta@example.io",
		"user.name":        "bob",
	}
	_ = UpdateLocal("/tmp/testconfig/.zeta", &UpdateOptions{Values: values})

	values["user.name"] = "Staff"
	_ = UpdateLocal("/tmp/testconfig/.zeta", &UpdateOptions{Values: values})
}

func TestEncodeInt(t *testing.T) {
	s := &Core{}
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	encoder.SetArraysMultiline(false)
	encoder.SetIndentTables(false)
	if err := encoder.Encode(s); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
}

func TestUpdateKey(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	doc, err := LoadDocumentFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		return
	}
	if err := doc.Add("core.sparse-checkout", "dev/jack"); err != nil {
		fmt.Fprintf(os.Stderr, "add error: %v\n", err)
		return
	}
	data, err := MarshalDocument(doc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s", data)
}

func TestUpdateNot(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	doc, err := LoadDocumentFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		return
	}
	if err := doc.Add("core.sparse-checkout", int64(10086)); err != nil {
		fmt.Fprintf(os.Stderr, "add error: %v\n", err)
		return
	}
	data, err := MarshalDocument(doc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s", data)
}

func TestUpdateNot2(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	doc, err := LoadDocumentFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		return
	}
	if err := doc.Add("core.namespace", int64(10086)); err != nil {
		fmt.Fprintf(os.Stderr, "add error: %v\n", err)
		return
	}
	data, err := MarshalDocument(doc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s", data)
}

func TestUpdateValidationFailure(t *testing.T) {
	// Create a temp file with valid content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "zeta.toml")

	originalContent := `[core]
editor = "vim"
`
	if err := os.WriteFile(tmpFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Read original content
	dataBefore, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	// Try to update with an invalid key (nested path)
	err = updateInternal(tmpFile, &UpdateOptions{
		Values: map[string]any{
			"a.b.c": "value",
		},
	})
	if err == nil {
		t.Errorf("updateInternal() expected error for bad key, got nil")
	}

	// Read content after failed update
	dataAfter, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	// Content should be unchanged
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Errorf("File content changed after failed update\nBefore: %s\nAfter: %s", dataBefore, dataAfter)
	}
}

func TestUnsetValidationFailure(t *testing.T) {
	// Create a temp file with valid content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "zeta.toml")

	originalContent := `[core]
editor = "vim"
`
	if err := os.WriteFile(tmpFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Read original content
	dataBefore, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	// Try to unset with an invalid key (nested path)
	err = unsetInternal(tmpFile, "a.b.c")
	if err == nil {
		t.Errorf("unsetInternal() expected error for bad key, got nil")
	}

	// Read content after failed unset
	dataAfter, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	// Content should be unchanged
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Errorf("File content changed after failed unset\nBefore: %s\nAfter: %s", dataBefore, dataAfter)
	}
}
