// Copyright (c) 2017- GitHub, Inc. and Git LFS contributors
// SPDX-License-Identifier: MIT

package pack

import (
	"errors"
	"fmt"
)

// UnsupportedVersionErr is a type implementing 'error' which indicates a
// the presence of an unsupported packfile version.
type UnsupportedVersionErr struct {
	// Got is the unsupported version that was detected.
	Got uint32
}

// Error implements 'error.Error()'.
func (u *UnsupportedVersionErr) Error() string {
	return fmt.Sprintf("zeta: unsupported version: %d", u.Got)
}

var (
	errBadPackHeader  = errors.New("zeta: bad pack header")
	errBadIndexHeader = errors.New("zeta: bad index header")
)
