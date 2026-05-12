package plumbing

import (
	"errors"
	"fmt"
)

var (
	//ErrStop is used to stop a ForEach function in an Iter
	ErrStop = errors.New("stop iter")
)

// noSuchObject is an error type that occurs when no object with a given object
// ID is available.
type noSuchObject struct {
	oid Hash
}

// Error implements the error.Error() function.
func (e *noSuchObject) Error() string {
	return fmt.Sprintf("zeta: no such object: %s", e.oid)
}

// NoSuchObject creates a new error representing a missing object with a given
// object ID.
func NoSuchObject(oid Hash) error {
	return &noSuchObject{oid: oid}
}

// IsNoSuchObject indicates whether an error is a noSuchObject and is non-nil.
func IsNoSuchObject(e error) bool {
	var err *noSuchObject
	return errors.As(e, &err)
}

func AsNoSuchObjectErr(e error) (Hash, bool) {
	if e, ok := errors.AsType[*noSuchObject](e); ok {
		return e.oid, true
	}
	return ZeroHash, false
}

type ErrResourceLocked struct {
	name ReferenceName
	t    string
}

func (err *ErrResourceLocked) Error() string {
	return fmt.Sprintf("%s '%s' locked", err.t, err.name)
}

func IsErrResourceLocked(err error) bool {
	var e *ErrResourceLocked
	return errors.As(err, &e)
}

func NewErrResourceLocked(t string, name ReferenceName) error {
	return &ErrResourceLocked{t: t, name: name}
}

type ErrRevNotFound struct {
	Reason string
}

func (e *ErrRevNotFound) Error() string { return e.Reason }

func NewErrRevNotFound(format string, a ...any) error {
	return &ErrRevNotFound{Reason: fmt.Sprintf(format, a...)}
}

func IsErrRevNotFound(err error) bool {
	var e *ErrRevNotFound
	return errors.As(err, &e)
}
