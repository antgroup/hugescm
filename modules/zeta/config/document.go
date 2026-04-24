// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"strings"
)

// Key represents a parsed configuration key with format "section.name".
type Key struct {
	Section string
	Name    string
}

// ParseKey parses a configuration key string into a Key struct.
// The key must be in the format "section.name".
// Returns ErrBadConfigKey for invalid formats.
func ParseKey(s string) (Key, error) {
	section, name, ok := strings.Cut(s, ".")
	if !ok {
		return Key{}, &ErrBadConfigKey{key: s}
	}
	if section == "" || name == "" {
		return Key{}, &ErrBadConfigKey{key: s}
	}
	// Check for nested dots (e.g., "a.b.c")
	if strings.Contains(name, ".") {
		return Key{}, &ErrBadConfigKey{key: s}
	}
	return Key{Section: section, Name: name}, nil
}

// String returns the string representation of the key.
func (k Key) String() string {
	return k.Section + "." + k.Name
}

// Section represents a configuration section with typed values.
type Section map[string]Value

// Document represents a configuration document with multiple sections.
type Document map[string]Section

// NewDocument creates a new empty Document.
func NewDocument() Document {
	return make(Document)
}

// Get retrieves a value by key.
// Returns the value, whether it exists, and an error if the key is invalid.
func (d Document) Get(key string) (Value, bool, error) {
	k, err := ParseKey(key)
	if err != nil {
		return Value{}, false, err
	}
	section, ok := d[k.Section]
	if !ok {
		return Value{}, false, nil
	}
	value, ok := section[k.Name]
	return value, ok, nil
}

// GetFirst retrieves the first value by key.
// For scalar values, returns the value itself.
// For slice values, returns the first element.
// Returns ErrKeyNotFound if the key doesn't exist or the slice is empty.
func (d Document) GetFirst(key string) (any, error) {
	value, exists, err := d.Get(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrKeyNotFound
	}
	first, ok := value.First()
	if !ok {
		return nil, ErrKeyNotFound
	}
	return first, nil
}

// GetAll retrieves all values by key.
// For scalar values, returns a single-element slice.
// For slice values, returns all elements.
// Returns ErrKeyNotFound if the key doesn't exist.
func (d Document) GetAll(key string) ([]any, error) {
	value, exists, err := d.Get(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrKeyNotFound
	}
	all := value.All()
	if all == nil {
		return nil, ErrKeyNotFound
	}
	return all, nil
}

// Set sets a value by key.
// Creates the section if it doesn't exist.
// Returns true if an existing value was overwritten.
func (d Document) Set(key string, val any) (bool, error) {
	k, err := ParseKey(key)
	if err != nil {
		return false, err
	}
	value, err := FromAny(val)
	if err != nil {
		return false, err
	}
	section, ok := d[k.Section]
	if !ok {
		section = make(Section)
		d[k.Section] = section
	}
	_, exists := section[k.Name]
	section[k.Name] = value
	return exists, nil
}

// Add appends a value to an existing key.
// If the key doesn't exist, creates a new single-element slice.
// If the key exists with a scalar value, converts to slice and appends.
// Returns an error for type mismatch.
func (d Document) Add(key string, val any) error {
	k, err := ParseKey(key)
	if err != nil {
		return err
	}
	newValue, err := FromAny(val)
	if err != nil {
		return err
	}

	section, ok := d[k.Section]
	if !ok {
		section = make(Section)
		d[k.Section] = section
	}

	existing, exists := section[k.Name]
	if !exists {
		// Key doesn't exist, set the new value directly
		section[k.Name] = newValue
		return nil
	}

	// Append to existing value
	combined, err := existing.Append(newValue)
	if err != nil {
		return fmt.Errorf("cannot add to key %q: %w", key, err)
	}
	section[k.Name] = combined
	return nil
}

// Delete removes a key from the document.
// Returns ErrKeyNotFound if the key doesn't exist.
func (d Document) Delete(key string) error {
	k, err := ParseKey(key)
	if err != nil {
		return err
	}
	section, ok := d[k.Section]
	if !ok {
		return ErrKeyNotFound
	}
	if _, ok := section[k.Name]; !ok {
		return ErrKeyNotFound
	}
	delete(section, k.Name)
	// Remove empty section
	if len(section) == 0 {
		delete(d, k.Section)
	}
	return nil
}

// Raw converts the Document to a map[string]map[string]any for encoding.
func (d Document) Raw() map[string]map[string]any {
	result := make(map[string]map[string]any)
	for sectionName, section := range d {
		rawSection := make(map[string]any)
		for keyName, value := range section {
			rawSection[keyName] = value.ToAny()
		}
		if len(rawSection) > 0 {
			result[sectionName] = rawSection
		}
	}
	return result
}

// FromRaw creates a Document from a map[string]map[string]any.
// Returns an error if any value has an unsupported type.
func FromRaw(raw map[string]map[string]any) (Document, error) {
	doc := make(Document)
	for sectionName, rawSection := range raw {
		section := make(Section)
		for keyName, rawValue := range rawSection {
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

// displayTo displays all values in the section to the Display interface.
func (s Section) displayTo(d Display, sectionKey string) error {
	for subKey, value := range s {
		if err := d.Show(value.ToAny(), sectionKey, subKey); err != nil {
			return err
		}
	}
	return nil
}
