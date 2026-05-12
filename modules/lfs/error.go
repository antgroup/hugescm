package lfs

import (
	"errors"
	"fmt"
)

type notAPointerError struct {
	message string
}

func (e *notAPointerError) Error() string {
	return fmt.Sprintf("Pointer file error: %v", e.message)
}

func NewNotAPointerError(message string) error {
	return &notAPointerError{message: message}
}

func IsNewNotAPointerError(err error) bool {
	var e *notAPointerError
	return errors.As(err, &e)
}

type badPointerKeyError struct {
	message string
}

func (e *badPointerKeyError) Error() string {
	return fmt.Sprintf("bad LFS Pointer: %v", e.message)
}

func NewBadPointerKeyError(message string) error {
	return &badPointerKeyError{message: message}
}

func IsBadPointerKeyError(err error) bool {
	var e *badPointerKeyError
	return errors.As(err, &e)
}
