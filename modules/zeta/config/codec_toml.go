// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// LoadDocument loads a Document from TOML bytes.
// It validates the TOML structure:
// - Top-level must be a table (map)
// - Each section must be a table (map)
// - No array of tables
// - No empty arrays (cannot infer type)
func LoadDocument(data []byte) (Document, error) {
	var raw map[string]any
	decoder := toml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	return fromRawAny(raw)
}

// fromRawAny converts a map[string]any to Document with validation.
func fromRawAny(raw map[string]any) (Document, error) {
	doc := make(Document)
	for sectionName, sectionValue := range raw {
		// Each top-level value must be a map (section)
		sectionMap, ok := sectionValue.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid TOML structure: top-level key %q is not a table", sectionName)
		}
		section := make(Section)
		for keyName, rawValue := range sectionMap {
			// Check for nested tables
			if _, isTable := rawValue.(map[string]any); isTable {
				return nil, fmt.Errorf("invalid TOML structure: nested table at %q.%q", sectionName, keyName)
			}
			// Check for array of tables
			if arr, isArray := rawValue.([]any); isArray && len(arr) > 0 {
				if _, isTable := arr[0].(map[string]any); isTable {
					return nil, fmt.Errorf("invalid TOML structure: array of tables at %q.%q not supported", sectionName, keyName)
				}
			}
			// Check for empty []any (cannot infer type)
			if arr, isArray := rawValue.([]any); isArray && len(arr) == 0 {
				return nil, fmt.Errorf("invalid TOML structure: empty array at %q.%q, cannot infer type", sectionName, keyName)
			}
			value, err := FromAny(rawValue)
			if err != nil {
				return nil, fmt.Errorf("section %q key %q: %w", sectionName, keyName, err)
			}
			section[keyName] = value
		}
		if len(section) > 0 {
			doc[sectionName] = section
		}
	}
	return doc, nil
}

// LoadDocumentFile loads a Document from a TOML file.
func LoadDocumentFile(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadDocument(data)
}

// MarshalDocument marshals a Document to TOML bytes.
func MarshalDocument(doc Document) ([]byte, error) {
	var buf bytes.Buffer
	encoder := newTOMLEncoder(&buf)
	if err := encoder.Encode(doc.Raw()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// newTOMLEncoder creates a TOML encoder with consistent configuration.
func newTOMLEncoder(w io.Writer) *toml.Encoder {
	encoder := toml.NewEncoder(w)
	encoder.SetArraysMultiline(false)
	encoder.SetIndentTables(false)
	return encoder
}

// LoadConfig loads TOML bytes into a Config struct.
func LoadConfig(data []byte, cfg *Config) error {
	decoder := toml.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(cfg)
}

// LoadConfigFile loads a TOML file into a Config struct.
func LoadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return LoadConfig(data, cfg)
}

// ValidateDocumentAs validates that a Document can be decoded into the provided struct.
// This is used to ensure that a Document represents a valid Config before writing.
func ValidateDocumentAs(doc Document, target any) error {
	data, err := MarshalDocument(doc)
	if err != nil {
		return err
	}
	return toml.NewDecoder(bytes.NewReader(data)).Decode(target)
}
