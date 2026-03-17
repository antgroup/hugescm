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
	}{
		// Boolean values
		{"bool true", true, BOOLEAN_TRUE},
		{"bool false", false, BOOLEAN_FALSE},
		// String values
		{"string true", "true", BOOLEAN_TRUE},
		{"string false", "false", BOOLEAN_FALSE},
		{"string yes", "yes", BOOLEAN_TRUE},
		{"string no", "no", BOOLEAN_FALSE},
		{"string on", "on", BOOLEAN_TRUE},
		{"string off", "off", BOOLEAN_FALSE},
		{"string 1", "1", BOOLEAN_TRUE},
		{"string 0", "0", BOOLEAN_FALSE},
		// Integer values
		{"int 1", int64(1), BOOLEAN_TRUE},
		{"int 0", int64(0), BOOLEAN_FALSE},
		// Case insensitive
		{"TRUE", "TRUE", BOOLEAN_TRUE},
		{"FALSE", "FALSE", BOOLEAN_FALSE},
		{"Yes", "Yes", BOOLEAN_TRUE},
		{"No", "No", BOOLEAN_FALSE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Boolean
			if err := b.UnmarshalTOML(tt.input); err != nil {
				t.Errorf("UnmarshalTOML(%v) error = %v", tt.input, err)
				return
			}
			if b.val != tt.expected {
				t.Errorf("UnmarshalTOML(%v) = %v, want %v", tt.input, b.val, tt.expected)
			}
		})
	}
}
