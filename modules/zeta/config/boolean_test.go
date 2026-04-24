// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestBooleanMerge(t *testing.T) {
	tests := []struct {
		name     string
		b        Boolean
		other    Boolean
		expected int
	}{
		{"UNSET + TRUE = TRUE", Boolean{val: BOOLEAN_UNSET}, Boolean{val: BOOLEAN_TRUE}, BOOLEAN_TRUE},
		{"UNSET + FALSE = FALSE", Boolean{val: BOOLEAN_UNSET}, Boolean{val: BOOLEAN_FALSE}, BOOLEAN_FALSE},
		{"UNSET + UNSET = UNSET", Boolean{val: BOOLEAN_UNSET}, Boolean{val: BOOLEAN_UNSET}, BOOLEAN_UNSET},
		{"TRUE + FALSE = FALSE (higher priority)", Boolean{val: BOOLEAN_TRUE}, Boolean{val: BOOLEAN_FALSE}, BOOLEAN_FALSE},
		{"FALSE + TRUE = TRUE (higher priority)", Boolean{val: BOOLEAN_FALSE}, Boolean{val: BOOLEAN_TRUE}, BOOLEAN_TRUE},
		{"TRUE + UNSET = TRUE", Boolean{val: BOOLEAN_TRUE}, Boolean{val: BOOLEAN_UNSET}, BOOLEAN_TRUE},
		{"FALSE + UNSET = FALSE", Boolean{val: BOOLEAN_FALSE}, Boolean{val: BOOLEAN_UNSET}, BOOLEAN_FALSE},
		{"TRUE + TRUE = TRUE", Boolean{val: BOOLEAN_TRUE}, Boolean{val: BOOLEAN_TRUE}, BOOLEAN_TRUE},
		{"FALSE + FALSE = FALSE", Boolean{val: BOOLEAN_FALSE}, Boolean{val: BOOLEAN_FALSE}, BOOLEAN_FALSE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.b
			b.Merge(&tt.other)
			if b.val != tt.expected {
				t.Errorf("Merge() = %v, want %v", b.val, tt.expected)
			}
		})
	}
}

func TestBooleanUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
		wantErr  bool
	}{
		// Boolean values
		{"bool true", true, BOOLEAN_TRUE, false},
		{"bool false", false, BOOLEAN_FALSE, false},
		// String values
		{"string true", "true", BOOLEAN_TRUE, false},
		{"string false", "false", BOOLEAN_FALSE, false},
		{"string yes", "yes", BOOLEAN_TRUE, false},
		{"string no", "no", BOOLEAN_FALSE, false},
		{"string on", "on", BOOLEAN_TRUE, false},
		{"string off", "off", BOOLEAN_FALSE, false},
		{"string 1", "1", BOOLEAN_TRUE, false},
		{"string 0", "0", BOOLEAN_FALSE, false},
		// Integer values
		{"int 1", int64(1), BOOLEAN_TRUE, false},
		{"int 0", int64(0), BOOLEAN_FALSE, false},
		// Case insensitive
		{"TRUE", "TRUE", BOOLEAN_TRUE, false},
		{"FALSE", "FALSE", BOOLEAN_FALSE, false},
		{"Yes", "Yes", BOOLEAN_TRUE, false},
		{"No", "No", BOOLEAN_FALSE, false},
		// Invalid values should error
		{"invalid string", "invalid", BOOLEAN_UNSET, true},
		{"invalid float", 3.14, BOOLEAN_UNSET, true},
		{"unsupported type", struct{}{}, BOOLEAN_UNSET, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Boolean
			err := b.UnmarshalTOML(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalTOML(%v) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("UnmarshalTOML(%v) error = %v", tt.input, err)
				return
			}
			if b.val != tt.expected {
				t.Errorf("UnmarshalTOML(%v) = %v, want %v", tt.input, b.val, tt.expected)
			}
		})
	}
}

func TestBooleanUnmarshalText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{"true", "true", BOOLEAN_TRUE, false},
		{"false", "false", BOOLEAN_FALSE, false},
		{"yes", "yes", BOOLEAN_TRUE, false},
		{"no", "no", BOOLEAN_FALSE, false},
		{"on", "on", BOOLEAN_TRUE, false},
		{"off", "off", BOOLEAN_FALSE, false},
		{"1", "1", BOOLEAN_TRUE, false},
		{"0", "0", BOOLEAN_FALSE, false},
		{"TRUE", "TRUE", BOOLEAN_TRUE, false},
		{"FALSE", "FALSE", BOOLEAN_FALSE, false},
		{"invalid", "invalid", BOOLEAN_UNSET, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Boolean
			err := b.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalText(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("UnmarshalText(%q) error = %v", tt.input, err)
				return
			}
			if b.val != tt.expected {
				t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, b.val, tt.expected)
			}
		})
	}
}
