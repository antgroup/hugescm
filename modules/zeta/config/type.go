// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"fmt"
	"reflect"
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
			return nil
		}
		b.val = BOOLEAN_FALSE
		return nil
	case int64:
		if data != 0 {
			b.val = BOOLEAN_TRUE
			return nil
		}
		b.val = BOOLEAN_FALSE
		return nil
	default:
	}
	switch strings.ToLower(s) {
	case "true", "yes", "on", "1":
		b.val = BOOLEAN_TRUE
	case "false", "no", "off", "0":
		b.val = BOOLEAN_FALSE
	}
	return nil
}

func (b *Boolean) IsUnset() bool {
	return b.val == BOOLEAN_UNSET
}

func (b *Boolean) Merge(other *Boolean) {
	if b.val == BOOLEAN_UNSET {
		b.val = other.val
	}
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

type Size struct {
	Size int64
}

func (s *Size) UnmarshalText(text []byte) (err error) {
	if bytes.HasSuffix(text, []byte("b")) || bytes.HasSuffix(text, []byte("B")) {
		text = text[0 : len(text)-1]
	}
	s.Size, err = strengthen.ParseSize(string(text))
	return
}

type Accelerator string

const (
	Direct    Accelerator = "direct"
	Dragonfly Accelerator = "dragonfly"
	Aria2     Accelerator = "aria2" // https://github.com/aria2/aria2
)

type Strategy string // Prune strategy

const (
	STRATEGY_UNSPECIFIED Strategy = "unspecified"
	STRATEGY_HEURISTICAL Strategy = "heuristical"
	STRATEGY_EAGER       Strategy = "eager"
	STRATEGY_EXTREME     Strategy = "extreme"
)

type Section map[string]any

type Display interface {
	Show(a any, keys ...string) error
}

func (s Section) displayTo(d Display, sectionKey string) error {
	for subKey, v := range s {
		if err := d.Show(v, sectionKey, subKey); err != nil {
			return err
		}
	}
	return nil
}

func (s Section) filter(k string) (any, error) {
	v, ok := s[k]
	if !ok {
		return nil, ErrKeyNotFound
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array:
		if rv.Len() == 0 {
			return nil, ErrKeyNotFound
		}
		return rv.Index(0).Interface(), nil
	case reflect.Slice:
		if rv.Len() == 0 {
			return nil, ErrKeyNotFound
		}
		return rv.Index(0).Interface(), nil
	default:
	}
	return v, nil
}

func (s Section) filterAll(k string) ([]any, error) {
	v, ok := s[k]
	if !ok {
		return nil, ErrKeyNotFound
	}
	vals := make([]any, 0, 4)
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			vals = append(vals, rv.Index(i).Interface())
		}
	case reflect.Slice:
		for i := 0; i < rv.Len(); i++ {
			vals = append(vals, rv.Index(i).Interface())
		}
	default:
		vals = append(vals, v)
	}
	return vals, nil
}

type Sections map[string]Section

func (ss Sections) filter(key string) (any, error) {
	sectionKey, subKey, ok := strings.Cut(key, ".")
	if !ok {
		return nil, &ErrBadConfigKey{key: key}
	}
	if s, ok := ss[sectionKey]; ok {
		return s.filter(subKey)
	}
	return nil, ErrKeyNotFound
}

func (ss Sections) filterAll(key string) ([]any, error) {
	sectionKey, subKey, ok := strings.Cut(key, ".")
	if !ok {
		return nil, &ErrBadConfigKey{key: key}
	}
	if s, ok := ss[sectionKey]; ok {
		return s.filterAll(subKey)
	}
	return nil, ErrKeyNotFound
}

func (ss Sections) deleteKey(key string) (bool, error) {
	sectionKey, subKey, ok := strings.Cut(key, ".")
	if !ok {
		return false, &ErrBadConfigKey{key: key}
	}
	s, ok := ss[sectionKey]
	if !ok {
		return false, ErrKeyNotFound
	}
	var deleted bool
	if _, ok := s[subKey]; ok {
		delete(s, subKey)
		deleted = true
	}
	if len(s) == 0 {
		delete(ss, sectionKey)
	}
	return deleted, nil
}

func valuesToStringArray(o any) []string {
	switch v := o.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		rv := make([]string, 0, len(v))
		for _, a := range v {
			rv = append(rv, valuesToStringArray(a)...)
		}
		return rv
	}
	return nil
}

func valuesToInt64Array(o any) []int64 {
	switch v := o.(type) {
	case int:
		return []int64{int64(v)}
	case int8:
		return []int64{int64(v)}
	case int16:
		return []int64{int64(v)}
	case int32:
		return []int64{int64(v)}
	case int64:
		return []int64{v}
	case []int64:
		return v
	case []any:
		rv := make([]int64, 0, len(v))
		for _, a := range v {
			rv = append(rv, valuesToInt64Array(a)...)
		}
		return rv
	}
	return nil
}

func valuesToBoolArray(o any) []bool {
	switch v := o.(type) {
	case string:
		return []bool{strengthen.SimpleAtob(v, false)}
	case bool:
		return []bool{v}
	case []any:
		values := make([]bool, 0, len(v)+1)
		for _, e := range v {
			if s, ok := e.(string); ok {
				values = append(values, strengthen.SimpleAtob(s, false))
				continue
			}
			if i, ok := e.(bool); ok {
				values = append(values, i)
				continue
			}
		}
		return values
	default:
	}
	return nil
}

func valuesToFloatArray(o any) []float64 {
	switch v := o.(type) {
	case float32:
		return []float64{float64(v)}
	case []float32:
		f64 := make([]float64, 0, len(v))
		for _, f := range v {
			f64 = append(f64, float64(f))
		}
		return f64
	case float64:
		return []float64{v}
	case []float64:
		return v
	case []any:
		rv := make([]float64, 0, len(v))
		for _, a := range v {
			rv = append(rv, valuesToFloatArray(a)...)
		}
		return rv
	}
	return nil
}

func valuesAppend(raw any, val any) any {
	switch nv := val.(type) {
	case string:
		rv := valuesToStringArray(raw)
		return append(rv, nv)
	case int64:
		rv := valuesToInt64Array(raw)
		return append(rv, nv)
	case bool:
		rv := valuesToBoolArray(raw)
		return append(rv, nv)
	case float64:
		rv := valuesToFloatArray(raw)
		return append(rv, nv)
	default:
	}
	return []any{val}
}

func (ss Sections) updateKey(key string, val any, append bool) (bool, error) {
	sectionKey, subKey, ok := strings.Cut(key, ".")
	if !ok {
		return false, &ErrBadConfigKey{key: key}
	}
	s, ok := ss[sectionKey]
	if !ok {
		newSection := make(Section)
		newSection[subKey] = val
		ss[sectionKey] = newSection
		return true, nil
	}
	if raw, ok := s[subKey]; ok && append {
		s[subKey] = valuesAppend(raw, val)
		return true, nil
	}
	s[subKey] = val
	return false, nil
}
