package errors

import (
	"errors"
	"fmt"
)

// noSuchObject is an error type that occurs when no object with a given object
// ID is available.
type noSuchObject struct {
	oid []byte
}

// Error implements the error.Error() function.
func (e *noSuchObject) Error() string {
	return fmt.Sprintf("git/object: no such object: %x", e.oid)
}

// NoSuchObject creates a new error representing a missing object with a given
// object ID.
func NoSuchObject(oid []byte) error {
	return &noSuchObject{oid: oid}
}

// IsNoSuchObject indicates whether an error is a noSuchObject and is non-nil.
func IsNoSuchObject(err error) bool {
	var e *noSuchObject
	return errors.As(err, &e)
}
