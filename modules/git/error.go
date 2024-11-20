package git

import (
	"fmt"
	"strings"
)

// ErrNotExist commit not exist error
type ErrNotExist struct {
	message string
}

// IsErrNotExist if some error is ErrNotExist
func IsErrNotExist(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrNotExist)
	return ok
}

func (err *ErrNotExist) Error() string {
	return err.message
}

func NewObjectNotFound(oid string) error {
	return &ErrNotExist{message: fmt.Sprintf("object '%s' does not exist", oid)}
}

func NewBranchNotFound(branch string) error {
	return &ErrNotExist{message: fmt.Sprintf("branch '%s' does not exist ", branch)}
}

var (
	ErrNoBranches = NewBranchNotFound("HEAD")
)

func NewTagNotFound(branch string) error {
	return &ErrNotExist{message: fmt.Sprintf("tag '%s' does not exist ", branch)}
}

func NewRevisionNotFound(branch string) error {
	return &ErrNotExist{message: fmt.Sprintf("revision '%s' does not exist ", branch)}
}

type ErrUnexpectedType struct {
	message string
}

func (e *ErrUnexpectedType) Error() string {
	return e.message
}

func IsErrUnexpectedType(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrUnexpectedType)
	return ok
}

var (
	notFoundPrefix = []string{
		"fatal: ambiguous argument",
		"fatal: unable to read",
		"fatal: bad object",
		"fatal: bad revision",
		//"fatal: unable to read tree",
	}
)

func ErrorIsNotFound(message string) bool {
	for _, s := range notFoundPrefix {
		if strings.HasPrefix(message, s) {
			return true
		}
	}
	return false
}
