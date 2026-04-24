// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestFromAny(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		wantKind  Kind
		wantValue any
		wantError bool
	}{
		// Scalar types
		{
			name:      "string",
			input:     "hello",
			wantKind:  KindString,
			wantValue: "hello",
		},
		{
			name:      "int",
			input:     42,
			wantKind:  KindInt64,
			wantValue: int64(42),
		},
		{
			name:      "int64",
			input:     int64(123),
			wantKind:  KindInt64,
			wantValue: int64(123),
		},
		{
			name:      "bool true",
			input:     true,
			wantKind:  KindBool,
			wantValue: true,
		},
		{
			name:      "bool false",
			input:     false,
			wantKind:  KindBool,
			wantValue: false,
		},
		{
			name:      "float64",
			input:     3.14,
			wantKind:  KindFloat64,
			wantValue: 3.14,
		},

		// Slice types
		{
			name:      "[]string",
			input:     []string{"a", "b", "c"},
			wantKind:  KindStringSlice,
			wantValue: []string{"a", "b", "c"},
		},
		{
			name:      "[]int64",
			input:     []int64{1, 2, 3},
			wantKind:  KindInt64Slice,
			wantValue: []int64{1, 2, 3},
		},
		{
			name:      "[]bool",
			input:     []bool{true, false},
			wantKind:  KindBoolSlice,
			wantValue: []bool{true, false},
		},
		{
			name:      "[]float64",
			input:     []float64{1.1, 2.2},
			wantKind:  KindFloat64Slice,
			wantValue: []float64{1.1, 2.2},
		},

		// []any same type
		{
			name:      "[]any string",
			input:     []any{"a", "b"},
			wantKind:  KindStringSlice,
			wantValue: []string{"a", "b"},
		},
		{
			name:      "[]any int64",
			input:     []any{int64(1), int64(2)},
			wantKind:  KindInt64Slice,
			wantValue: []int64{1, 2},
		},

		// Mixed type error
		{
			name:      "[]any mixed types",
			input:     []any{"a", 1},
			wantError: true,
		},

		// Unsupported type
		{
			name:      "unsupported type",
			input:     struct{}{},
			wantError: true,
		},

		// Empty []any should error
		{
			name:      "empty []any",
			input:     []any{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := FromAny(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("FromAny(%v) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("FromAny(%v) unexpected error: %v", tt.input, err)
				return
			}
			if v.Kind() != tt.wantKind {
				t.Errorf("FromAny(%v) kind = %v, want %v", tt.input, v.Kind(), tt.wantKind)
			}
			// Compare values
			got := v.ToAny()
			if !compareValues(got, tt.wantValue) {
				t.Errorf("FromAny(%v) = %v, want %v", tt.input, got, tt.wantValue)
			}
		})
	}
}

func TestValueAppend(t *testing.T) {
	tests := []struct {
		name      string
		v1        Value
		v2        Value
		wantKind  Kind
		wantValue any
		wantError bool
	}{
		// scalar + scalar -> slice
		{
			name:      "string + string",
			v1:        NewStringValue("a"),
			v2:        NewStringValue("b"),
			wantKind:  KindStringSlice,
			wantValue: []string{"a", "b"},
		},
		{
			name:      "int64 + int64",
			v1:        NewInt64Value(1),
			v2:        NewInt64Value(2),
			wantKind:  KindInt64Slice,
			wantValue: []int64{1, 2},
		},
		{
			name:      "bool + bool",
			v1:        NewBoolValue(true),
			v2:        NewBoolValue(false),
			wantKind:  KindBoolSlice,
			wantValue: []bool{true, false},
		},

		// slice + scalar -> slice
		{
			name:      "[]string + string",
			v1:        NewStringSliceValue([]string{"a", "b"}),
			v2:        NewStringValue("c"),
			wantKind:  KindStringSlice,
			wantValue: []string{"a", "b", "c"},
		},
		{
			name:      "[]int64 + int64",
			v1:        NewInt64SliceValue([]int64{1, 2}),
			v2:        NewInt64Value(3),
			wantKind:  KindInt64Slice,
			wantValue: []int64{1, 2, 3},
		},

		// Type mismatch error
		{
			name:      "string + int64",
			v1:        NewStringValue("a"),
			v2:        NewInt64Value(1),
			wantError: true,
		},
		{
			name:      "[]string + int64",
			v1:        NewStringSliceValue([]string{"a"}),
			v2:        NewInt64Value(1),
			wantError: true,
		},

		// slice + slice -> error
		{
			name:      "[]string + []string",
			v1:        NewStringSliceValue([]string{"a"}),
			v2:        NewStringSliceValue([]string{"b"}),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.v1.Append(tt.v2)
			if tt.wantError {
				if err == nil {
					t.Errorf("Append() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Append() unexpected error: %v", err)
				return
			}
			if result.Kind() != tt.wantKind {
				t.Errorf("Append() kind = %v, want %v", result.Kind(), tt.wantKind)
			}
			got := result.ToAny()
			if !compareValues(got, tt.wantValue) {
				t.Errorf("Append() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

func TestValueFirstAll(t *testing.T) {
	tests := []struct {
		name        string
		value       Value
		wantFirst   any
		wantFirstOk bool
		wantAll     []any
	}{
		{
			name:        "string scalar",
			value:       NewStringValue("hello"),
			wantFirst:   "hello",
			wantFirstOk: true,
			wantAll:     []any{"hello"},
		},
		{
			name:        "int64 scalar",
			value:       NewInt64Value(42),
			wantFirst:   int64(42),
			wantFirstOk: true,
			wantAll:     []any{int64(42)},
		},
		{
			name:        "[]string slice",
			value:       NewStringSliceValue([]string{"a", "b", "c"}),
			wantFirst:   "a",
			wantFirstOk: true,
			wantAll:     []any{"a", "b", "c"},
		},
		{
			name:        "[]int64 slice",
			value:       NewInt64SliceValue([]int64{1, 2, 3}),
			wantFirst:   int64(1),
			wantFirstOk: true,
			wantAll:     []any{int64(1), int64(2), int64(3)},
		},
		{
			name:        "empty slice",
			value:       NewStringSliceValue([]string{}),
			wantFirst:   nil,
			wantFirstOk: false,
			wantAll:     []any{},
		},
		{
			name:        "invalid value",
			value:       Value{},
			wantFirst:   nil,
			wantFirstOk: false,
			wantAll:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFirst, gotOk := tt.value.First()
			if gotOk != tt.wantFirstOk {
				t.Errorf("First() ok = %v, want %v", gotOk, tt.wantFirstOk)
			}
			if !compareValues(gotFirst, tt.wantFirst) {
				t.Errorf("First() = %v, want %v", gotFirst, tt.wantFirst)
			}

			gotAll := tt.value.All()
			if !compareSlices(gotAll, tt.wantAll) {
				t.Errorf("All() = %v, want %v", gotAll, tt.wantAll)
			}
		})
	}
}

func TestValueSliceCopy(t *testing.T) {
	// Test that slice constructors copy the input slice
	original := []string{"a", "b", "c"}
	v := NewStringSliceValue(original)

	// Modify original
	original[0] = "modified"

	// Value should not be affected
	got := v.ToAny().([]string)
	if got[0] != "a" {
		t.Errorf("NewStringSliceValue did not copy: got %v, want 'a'", got[0])
	}

	// Test that Append creates a new slice
	v1 := NewStringSliceValue([]string{"a", "b"})
	v2 := NewStringValue("c")
	result, err := v1.Append(v2)
	if err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Modify original v1's underlying slice
	v1Slice := v1.ToAny().([]string)
	v1Slice[0] = "modified"

	// Result should not be affected
	resultSlice := result.ToAny().([]string)
	if resultSlice[0] != "a" {
		t.Errorf("Append did not copy: got %v, want 'a'", resultSlice[0])
	}
}

// Helper functions for comparison
func compareValues(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch a := a.(type) {
	case []string:
		bb, ok := b.([]string)
		if !ok {
			return false
		}
		if len(a) != len(bb) {
			return false
		}
		for i := range a {
			if a[i] != bb[i] {
				return false
			}
		}
		return true
	case []int64:
		bb, ok := b.([]int64)
		if !ok {
			return false
		}
		if len(a) != len(bb) {
			return false
		}
		for i := range a {
			if a[i] != bb[i] {
				return false
			}
		}
		return true
	case []bool:
		bb, ok := b.([]bool)
		if !ok {
			return false
		}
		if len(a) != len(bb) {
			return false
		}
		for i := range a {
			if a[i] != bb[i] {
				return false
			}
		}
		return true
	case []float64:
		bb, ok := b.([]float64)
		if !ok {
			return false
		}
		if len(a) != len(bb) {
			return false
		}
		for i := range a {
			if a[i] != bb[i] {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

func compareSlices(a, b []any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !compareValues(a[i], b[i]) {
			return false
		}
	}
	return true
}
