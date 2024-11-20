package lfs

import "fmt"

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
	if err == nil {
		return false
	}
	_, ok := err.(*notAPointerError)
	return ok
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
	if err == nil {
		return false
	}
	_, ok := err.(*badPointerKeyError)
	return ok
}
