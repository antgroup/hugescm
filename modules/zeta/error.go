// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedCompressMethod = errors.New("unsupported compress method")
	ErrMistakeHashText           = errors.New("mistake hash text")
	ErrUnsupportedObject         = errors.New("unsupported object type")
	ErrMismatchedMagic           = errors.New("mismatched magic")
	ErrMismatchedVersion         = errors.New("mismatched version")
)

type ErrMismatchedObject struct {
	Want string
	Got  string
}

func (err *ErrMismatchedObject) Error() string {
	return fmt.Sprintf("mismatched object want '%s' got '%s'", err.Want, err.Got)
}

func IsErrMismatchedObject(err error) bool {
	var e *ErrMismatchedObject
	return errors.As(err, &e)
}

type ErrNotExist struct {
	T   string
	OID string
}

func (err *ErrNotExist) Error() string {
	return fmt.Sprintf("%s '%s' not exist", err.T, err.OID)
}

func NewErrNotExist(t string, oid string) error {
	return &ErrNotExist{T: t, OID: oid}
}

func IsErrNotExist(err error) bool {
	if errors.Is(err, ErrMistakeHashText) {
		// NOT FOUND
		return true
	}
	var e *ErrNotExist
	return errors.As(err, &e)
}

type ErrStatusCode struct {
	Code    int
	Message string
}

func (err *ErrStatusCode) Error() string {
	return err.Message
}

func IsErrStatusCode(err error) bool {
	var e *ErrStatusCode
	return errors.As(err, &e)
}

func NewErrStatusCode(statusCode int, format string, a ...any) error {
	return &ErrStatusCode{Code: statusCode, Message: fmt.Sprintf(format, a...)}
}

type ErrExitCode struct {
	Code    int
	Message string
}

func (err *ErrExitCode) Error() string {
	return err.Message
}

func IsErrExitCode(err error) bool {
	var e *ErrExitCode
	return errors.As(err, &e)
}
