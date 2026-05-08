package types

import (
	"errors"
	"fmt"
	"io"
	"reflect"
)

// ScanFully uses fmt.Sscanf with verb to fully scan val into ptr.
func ScanFully(ptr any, val string, verb byte) error {
	t := reflect.ValueOf(ptr).Elem().Type()
	// attempt to read extra bytes to make sure the value is consumed
	var b []byte
	n, err := fmt.Sscanf(val, "%"+string(verb)+"%s", ptr, &b)
	switch {
	case n < 1 || n == 1 && !errors.Is(err, io.EOF):
		return fmt.Errorf("failed to parse %q as %v: %w", val, t, err)
	case n > 1:
		return fmt.Errorf("failed to parse %q as %v: extra characters %q", val, t, string(b))
	}
	// n == 1 && err == io.EOF
	return nil
}
