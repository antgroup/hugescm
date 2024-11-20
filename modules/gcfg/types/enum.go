package types

import (
	"fmt"
	"reflect"
	"strings"
)

// EnumParser parses "enum" values; i.e. a predefined set of strings to
// predefined values.
type EnumParser struct {
	Type      string // type name; if not set, use type of first value added
	CaseMatch bool   // if true, matching of strings is case-sensitive
	// PrefixMatch bool
	vals map[string]any
}

// AddVals adds strings and values to an EnumParser.
func (ep *EnumParser) AddVals(vals map[string]any) {
	if ep.vals == nil {
		ep.vals = make(map[string]any)
	}
	for k, v := range vals {
		if ep.Type == "" {
			ep.Type = reflect.TypeOf(v).Name()
		}
		if !ep.CaseMatch {
			k = strings.ToLower(k)
		}
		ep.vals[k] = v
	}
}

// Parse parses the string and returns the value or an error.
func (ep EnumParser) Parse(s string) (any, error) {
	if !ep.CaseMatch {
		s = strings.ToLower(s)
	}
	v, ok := ep.vals[s]
	if !ok {
		return false, fmt.Errorf("failed to parse %s %#q", ep.Type, s)
	}
	return v, nil
}
