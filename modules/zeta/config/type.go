// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/antgroup/hugescm/modules/strengthen"
)

const (
	UNSPECIFIED = ""
	BOOLEAN     = "bool"
	INTEGER     = "int"
	BOOLORINT   = "bool-or-int"
	PATH        = "path"
	DATETIME    = "datetime"
)

const (
	BOOLEAN_UNSET = 0
	BOOLEAN_TRUE  = 1
	BOOLEAN_FALSE = 2
)

type Boolean struct {
	val int
}

var (
	True  = Boolean{val: BOOLEAN_TRUE}
	False = Boolean{val: BOOLEAN_FALSE}
)

func (b *Boolean) UnmarshalTOML(a any) error {
	var s string
	switch data := a.(type) {
	case fmt.Stringer:
		s = data.String()
	case string:
		s = data
	case bool:
		if data {
			b.val = BOOLEAN_TRUE
		} else {
			b.val = BOOLEAN_FALSE
		}
		return nil
	case int64:
		if data != 0 {
			b.val = BOOLEAN_TRUE
		} else {
			b.val = BOOLEAN_FALSE
		}
		return nil
	case int:
		if data != 0 {
			b.val = BOOLEAN_TRUE
		} else {
			b.val = BOOLEAN_FALSE
		}
		return nil
	default:
		return fmt.Errorf("invalid boolean value: %T", a)
	}
	switch strings.ToLower(s) {
	case "true", "yes", "on", "1":
		b.val = BOOLEAN_TRUE
	case "false", "no", "off", "0":
		b.val = BOOLEAN_FALSE
	default:
		return fmt.Errorf("invalid boolean value: %q", s)
	}
	return nil
}

// UnmarshalText implements encoding.TextUnmarshaler for Boolean.
// This is used by go-toml/v2 for decoding boolean values.
func (b *Boolean) UnmarshalText(text []byte) error {
	s := strings.ToLower(string(text))
	switch s {
	case "true", "yes", "on", "1":
		b.val = BOOLEAN_TRUE
	case "false", "no", "off", "0":
		b.val = BOOLEAN_FALSE
	default:
		return fmt.Errorf("invalid boolean value: %q", string(text))
	}
	return nil
}

func (b *Boolean) IsUnset() bool {
	return b.val == BOOLEAN_UNSET
}

// Merge merges the other boolean value into b.
// If other has a definite value (TRUE or FALSE), it overrides b's value.
// This follows the config priority: local > global > system.
func (b *Boolean) Merge(other *Boolean) {
	// If other has a definite value, it should override b (higher priority)
	if other.val != BOOLEAN_UNSET {
		b.val = other.val
	}
	// If other is UNSET, keep b's current value (don't override with UNSET)
}

func (b *Boolean) True() bool {
	return b.val == BOOLEAN_TRUE
}

func (b *Boolean) False() bool {
	return b.val == BOOLEAN_FALSE
}

func (b *Boolean) Set(v bool) bool {
	if v {
		b.val = BOOLEAN_TRUE
		return true
	}
	b.val = BOOLEAN_FALSE
	return false
}

func (b *Boolean) Unset() {
	b.val = BOOLEAN_UNSET
}

// MarshalText implements encoding.TextMarshaler for Boolean.
// This is used by TOML encoder to convert Boolean to text representation.
func (b Boolean) MarshalText() ([]byte, error) {
	switch b.val {
	case BOOLEAN_TRUE:
		return []byte("true"), nil
	case BOOLEAN_FALSE:
		return []byte("false"), nil
	default:
		// UNSET - return empty string (will be handled by omitempty)
		return []byte(""), nil
	}
}

type StringArray []string

func (a *StringArray) UnmarshalTOML(data any) error {
	switch v := data.(type) {
	case string:
		*a = []string{v}
	case []any:
		var vv []string
		for _, e := range v {
			if s, ok := e.(string); ok {
				vv = append(vv, s)
				continue
			}
			return fmt.Errorf("expected string in array, but got %T", e)
		}
		*a = vv
	default:
		return fmt.Errorf("unexpected type %T", data)
	}
	return nil
}

type Size int64

func (s *Size) UnmarshalText(text []byte) error {
	if bytes.HasSuffix(text, []byte("b")) || bytes.HasSuffix(text, []byte("B")) {
		text = text[0 : len(text)-1]
	}
	size, err := strengthen.ParseSize(string(text))
	*s = Size(size)
	return err
}

type Accelerator string

const (
	Direct    Accelerator = "direct"
	Dragonfly Accelerator = "dragonfly"
	Aria2     Accelerator = "aria2" // https://github.com/aria2/aria2
)

type Strategy string // Prune strategy

const (
	StrategyUnspecified Strategy = "unspecified"
	StrategyHeuristical Strategy = "heuristical"
	StrategyEager       Strategy = "eager"
	StrategyExtreme     Strategy = "extreme"
)

type Display interface {
	Show(a any, keys ...string) error
}
