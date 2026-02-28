// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

const (
	ER_ACCESS_DENIED_ERROR = 1045
	ER_DUP_ENTRY           = 1062
)

var (
	ErrReferenceNotAllowed = errors.New("reference types not allowed")
	ErrUserNotGiven        = errors.New("user not given")
)

type ErrRevisionNotFound struct {
	Revision string
}

func (err *ErrRevisionNotFound) Error() string {
	return fmt.Sprintf("revision '%s' not found", err.Revision)
}

func IsErrRevisionNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrRevisionNotFound)
	return ok
}

func IsErrorCode(err error, code uint16) bool {
	if merr, ok := errors.AsType[*mysql.MySQLError](err); ok {
		return merr.Number == code
	}
	return false
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*ErrRevisionNotFound); ok {
		return true
	}
	return errors.Is(err, sql.ErrNoRows)
}

func IsDupEntry(err error) bool {
	return IsErrorCode(err, ER_DUP_ENTRY)
}

type ErrAlreadyLocked struct {
	Reference string
}

func (e *ErrAlreadyLocked) Error() string {
	return fmt.Sprintf("reference is already locked: %q", e.Reference)
}

func IsErrAlreadyLocked(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrAlreadyLocked)
	return ok
}

type ErrNamingRule struct {
	name string
}

func (e *ErrNamingRule) Error() string {
	return fmt.Sprintf("'%s' does not comply with the naming rules", e.name)
}

func IsErrNamingRule(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrNamingRule)
	return ok
}

type ErrExist struct {
	message string
}

func (e *ErrExist) Error() string {
	return e.message
}

func IsErrExist(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrExist)
	return ok
}
