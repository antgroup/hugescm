// Copyright (c) 2017- GitHub, Inc. and Git LFS contributors
// SPDX-License-Identifier: MIT

package backend

import (
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
	if err == nil {
		return false
	}
	_, ok := err.(*ErrMismatchedObjectType)
	return ok
}

func NewErrMismatchedObjectType(oid plumbing.Hash, t string) error {
	return &ErrMismatchedObjectType{oid: oid, t: t}
}
