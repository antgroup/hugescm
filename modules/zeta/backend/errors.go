// Copyright (c) 2017- GitHub, Inc. and Git LFS contributors
// SPDX-License-Identifier: MIT

package backend

import (
	"errors"
	"fmt"

	"github.com/antgroup/hugescm/modules/plumbing"
)

type ErrMismatchedObjectType struct {
	oid plumbing.Hash
	t   string
}

func (e *ErrMismatchedObjectType) Error() string {
	return fmt.Sprintf("object %s not %s", e.oid, e.t)
}

func IsErrMismatchedObjectType(err error) bool {
	var e *ErrMismatchedObjectType
	return errors.As(err, &e)
}

func NewErrMismatchedObjectType(oid plumbing.Hash, t string) error {
	return &ErrMismatchedObjectType{oid: oid, t: t}
}
