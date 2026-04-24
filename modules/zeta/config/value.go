// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
)

// Kind represents the type of a Value.
type Kind int

const (
	KindInvalid Kind = iota
	KindString
	KindInt64
	KindBool
	KindFloat64
	KindStringSlice
	KindInt64Slice
	KindBoolSlice
	KindFloat64Slice
)

// String returns the string representation of Kind.
func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid"
	case KindString:
		return "string"
	case KindInt64:
		return "int64"
	case KindBool:
		return "bool"
	case KindFloat64:
		return "float64"
	case KindStringSlice:
		return "[]string"
	case KindInt64Slice:
		return "[]int64"
	case KindBoolSlice:
		return "[]bool"
	case KindFloat64Slice:
		return "[]float64"
	default:
		return "unknown"
	}
}

// Value represents a typed configuration value.
// It is the core value model for the dynamic editing layer.
// Zero value is invalid and should not be used.
type Value struct {
	kind  Kind
	value any
}

// NewStringValue creates a string Value.
func NewStringValue(s string) Value {
	return Value{kind: KindString, value: s}
}

// NewInt64Value creates an int64 Value.
func NewInt64Value(i int64) Value {
	return Value{kind: KindInt64, value: i}
}

// NewBoolValue creates a bool Value.
func NewBoolValue(b bool) Value {
	return Value{kind: KindBool, value: b}
}

// NewFloat64Value creates a float64 Value.
func NewFloat64Value(f float64) Value {
	return Value{kind: KindFloat64, value: f}
}

// NewStringSliceValue creates a []string Value.
// Makes a copy of the input slice to avoid sharing the underlying array.
func NewStringSliceValue(s []string) Value {
	if s == nil {
		s = []string{}
	}
	// Copy to avoid sharing underlying array
	copied := make([]string, len(s))
	copy(copied, s)
	return Value{kind: KindStringSlice, value: copied}
}

// NewInt64SliceValue creates a []int64 Value.
// Makes a copy of the input slice to avoid sharing the underlying array.
func NewInt64SliceValue(i []int64) Value {
	if i == nil {
		i = []int64{}
	}
	// Copy to avoid sharing underlying array
	copied := make([]int64, len(i))
	copy(copied, i)
	return Value{kind: KindInt64Slice, value: copied}
}

// NewBoolSliceValue creates a []bool Value.
// Makes a copy of the input slice to avoid sharing the underlying array.
func NewBoolSliceValue(b []bool) Value {
	if b == nil {
		b = []bool{}
	}
	// Copy to avoid sharing underlying array
	copied := make([]bool, len(b))
	copy(copied, b)
	return Value{kind: KindBoolSlice, value: copied}
}

// NewFloat64SliceValue creates a []float64 Value.
// Makes a copy of the input slice to avoid sharing the underlying array.
func NewFloat64SliceValue(f []float64) Value {
	if f == nil {
		f = []float64{}
	}
	// Copy to avoid sharing underlying array
	copied := make([]float64, len(f))
	copy(copied, f)
	return Value{kind: KindFloat64Slice, value: copied}
}

// Kind returns the kind of the value.
func (v Value) Kind() Kind {
	return v.kind
}

// IsZero returns true if the value is zero/invalid.
func (v Value) IsZero() bool {
	return v.kind == KindInvalid
}

// FromAny creates a Value from an any type.
// Returns an error if the type is not supported or if the slice contains mixed types.
func FromAny(a any) (Value, error) {
	switch val := a.(type) {
	case string:
		return NewStringValue(val), nil
	case int:
		return NewInt64Value(int64(val)), nil
	case int8:
		return NewInt64Value(int64(val)), nil
	case int16:
		return NewInt64Value(int64(val)), nil
	case int32:
		return NewInt64Value(int64(val)), nil
	case int64:
		return NewInt64Value(val), nil
	case bool:
		return NewBoolValue(val), nil
	case float32:
		return NewFloat64Value(float64(val)), nil
	case float64:
		return NewFloat64Value(val), nil
	case []string:
		return NewStringSliceValue(val), nil
	case []int64:
		return NewInt64SliceValue(val), nil
	case []bool:
		return NewBoolSliceValue(val), nil
	case []float64:
		return NewFloat64SliceValue(val), nil
	case []any:
		// Convert []any to typed slice
		if len(val) == 0 {
			// Empty []any cannot infer type
			return Value{}, errors.New("empty []any cannot infer type")
		}
		// Infer type from first element
		switch val[0].(type) {
		case string:
			slice := make([]string, 0, len(val))
			for i, elem := range val {
				s, ok := elem.(string)
				if !ok {
					return Value{}, fmt.Errorf("mixed types in slice: element 0 is string, element %d is %T", i, elem)
				}
				slice = append(slice, s)
			}
			return NewStringSliceValue(slice), nil
		case int:
			slice := make([]int64, 0, len(val))
			for i, elem := range val {
				n, ok := elem.(int)
				if !ok {
					return Value{}, fmt.Errorf("mixed types in slice: element 0 is int, element %d is %T", i, elem)
				}
				slice = append(slice, int64(n))
			}
			return NewInt64SliceValue(slice), nil
		case int64:
			slice := make([]int64, 0, len(val))
			for i, elem := range val {
				n, ok := elem.(int64)
				if !ok {
					return Value{}, fmt.Errorf("mixed types in slice: element 0 is int64, element %d is %T", i, elem)
				}
				slice = append(slice, n)
			}
			return NewInt64SliceValue(slice), nil
		case bool:
			slice := make([]bool, 0, len(val))
			for i, elem := range val {
				b, ok := elem.(bool)
				if !ok {
					return Value{}, fmt.Errorf("mixed types in slice: element 0 is bool, element %d is %T", i, elem)
				}
				slice = append(slice, b)
			}
			return NewBoolSliceValue(slice), nil
		case float64:
			slice := make([]float64, 0, len(val))
			for i, elem := range val {
				f, ok := elem.(float64)
				if !ok {
					return Value{}, fmt.Errorf("mixed types in slice: element 0 is float64, element %d is %T", i, elem)
				}
				slice = append(slice, f)
			}
			return NewFloat64SliceValue(slice), nil
		default:
			return Value{}, fmt.Errorf("unsupported slice element type: %T", val[0])
		}
	default:
		return Value{}, fmt.Errorf("unsupported type: %T", a)
	}
}

// ToAny returns the underlying value as any.
func (v Value) ToAny() any {
	switch v.kind {
	case KindString:
		return v.value.(string)
	case KindInt64:
		return v.value.(int64)
	case KindBool:
		return v.value.(bool)
	case KindFloat64:
		return v.value.(float64)
	case KindStringSlice:
		return v.value.([]string)
	case KindInt64Slice:
		return v.value.([]int64)
	case KindBoolSlice:
		return v.value.([]bool)
	case KindFloat64Slice:
		return v.value.([]float64)
	default:
		return nil
	}
}

// First returns the first element for slice values, or the value itself for scalar values.
// Returns false if the value is invalid or the slice is empty.
func (v Value) First() (any, bool) {
	switch v.kind {
	case KindString:
		return v.value.(string), true
	case KindInt64:
		return v.value.(int64), true
	case KindBool:
		return v.value.(bool), true
	case KindFloat64:
		return v.value.(float64), true
	case KindStringSlice:
		slice := v.value.([]string)
		if len(slice) == 0 {
			return nil, false
		}
		return slice[0], true
	case KindInt64Slice:
		slice := v.value.([]int64)
		if len(slice) == 0 {
			return nil, false
		}
		return slice[0], true
	case KindBoolSlice:
		slice := v.value.([]bool)
		if len(slice) == 0 {
			return nil, false
		}
		return slice[0], true
	case KindFloat64Slice:
		slice := v.value.([]float64)
		if len(slice) == 0 {
			return nil, false
		}
		return slice[0], true
	default:
		return nil, false
	}
}

// All returns all elements as []any.
// For scalar values, returns a single-element slice.
// For invalid values, returns nil.
func (v Value) All() []any {
	switch v.kind {
	case KindString:
		return []any{v.value.(string)}
	case KindInt64:
		return []any{v.value.(int64)}
	case KindBool:
		return []any{v.value.(bool)}
	case KindFloat64:
		return []any{v.value.(float64)}
	case KindStringSlice:
		slice := v.value.([]string)
		result := make([]any, len(slice))
		for i, s := range slice {
			result[i] = s
		}
		return result
	case KindInt64Slice:
		slice := v.value.([]int64)
		result := make([]any, len(slice))
		for i, n := range slice {
			result[i] = n
		}
		return result
	case KindBoolSlice:
		slice := v.value.([]bool)
		result := make([]any, len(slice))
		for i, b := range slice {
			result[i] = b
		}
		return result
	case KindFloat64Slice:
		slice := v.value.([]float64)
		result := make([]any, len(slice))
		for i, f := range slice {
			result[i] = f
		}
		return result
	default:
		return nil
	}
}

// Append appends another value to this value.
// Both values must be of compatible types.
// Scalar + Scalar -> typed slice
// Slice + Scalar -> typed slice (append)
// Slice + Slice -> error (not supported)
// Returns error for type mismatch.
func (v Value) Append(other Value) (Value, error) {
	if other.IsZero() {
		return v, nil
	}
	if v.IsZero() {
		return other, nil
	}

	// Both are scalars of the same type -> create slice
	if v.isScalar() && other.isScalar() {
		if v.kind != other.kind {
			return Value{}, fmt.Errorf("type mismatch: cannot append %s to %s", other.kind, v.kind)
		}
		switch v.kind {
		case KindString:
			return NewStringSliceValue([]string{v.value.(string), other.value.(string)}), nil
		case KindInt64:
			return NewInt64SliceValue([]int64{v.value.(int64), other.value.(int64)}), nil
		case KindBool:
			return NewBoolSliceValue([]bool{v.value.(bool), other.value.(bool)}), nil
		case KindFloat64:
			return NewFloat64SliceValue([]float64{v.value.(float64), other.value.(float64)}), nil
		}
	}

	// v is slice, other is scalar -> append
	if v.isSlice() && other.isScalar() {
		elementKind := v.sliceElementKind()
		if other.kind != elementKind {
			return Value{}, fmt.Errorf("type mismatch: cannot append %s to %s", other.kind, v.kind)
		}
		switch v.kind {
		case KindStringSlice:
			oldSlice := v.value.([]string)
			newSlice := make([]string, len(oldSlice)+1)
			copy(newSlice, oldSlice)
			newSlice[len(oldSlice)] = other.value.(string)
			return Value{kind: KindStringSlice, value: newSlice}, nil
		case KindInt64Slice:
			oldSlice := v.value.([]int64)
			newSlice := make([]int64, len(oldSlice)+1)
			copy(newSlice, oldSlice)
			newSlice[len(oldSlice)] = other.value.(int64)
			return Value{kind: KindInt64Slice, value: newSlice}, nil
		case KindBoolSlice:
			oldSlice := v.value.([]bool)
			newSlice := make([]bool, len(oldSlice)+1)
			copy(newSlice, oldSlice)
			newSlice[len(oldSlice)] = other.value.(bool)
			return Value{kind: KindBoolSlice, value: newSlice}, nil
		case KindFloat64Slice:
			oldSlice := v.value.([]float64)
			newSlice := make([]float64, len(oldSlice)+1)
			copy(newSlice, oldSlice)
			newSlice[len(oldSlice)] = other.value.(float64)
			return Value{kind: KindFloat64Slice, value: newSlice}, nil
		}
	}

	// v is scalar, other is slice -> error (cannot append slice to scalar)
	if v.isScalar() && other.isSlice() {
		return Value{}, errors.New("cannot append slice to scalar")
	}

	// Both are slices -> error (not supported in current semantics)
	if v.isSlice() && other.isSlice() {
		return Value{}, errors.New("cannot append slice to slice")
	}

	return Value{}, errors.New("unsupported append operation")
}

// isScalar returns true if the value is a scalar type.
func (v Value) isScalar() bool {
	switch v.kind {
	case KindString, KindInt64, KindBool, KindFloat64:
		return true
	default:
		return false
	}
}

// isSlice returns true if the value is a slice type.
func (v Value) isSlice() bool {
	switch v.kind {
	case KindStringSlice, KindInt64Slice, KindBoolSlice, KindFloat64Slice:
		return true
	default:
		return false
	}
}

// sliceElementKind returns the Kind of slice elements.
func (v Value) sliceElementKind() Kind {
	switch v.kind {
	case KindStringSlice:
		return KindString
	case KindInt64Slice:
		return KindInt64
	case KindBoolSlice:
		return KindBool
	case KindFloat64Slice:
		return KindFloat64
	default:
		return KindInvalid
	}
}
