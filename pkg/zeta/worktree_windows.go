// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package zeta

import (
	"os"
	"syscall"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

const (
	escapeChars = "*?[]"
)

func init() {
	fillSystemInfo = func(e *index.Entry, sys any) {
		if os, ok := sys.(*syscall.Win32FileAttributeData); ok {
			seconds := os.CreationTime.Nanoseconds() / 1000000000
			nanoseconds := os.CreationTime.Nanoseconds() - seconds*1000000000
			e.CreatedAt = time.Unix(seconds, nanoseconds)
		}
	}
}

func isSymlinkWindowsNonAdmin(err error) bool {
	const ERROR_PRIVILEGE_NOT_HELD syscall.Errno = 1314

	if err != nil {
		if errLink, ok := err.(*os.LinkError); ok {
			if errNo, ok := errLink.Err.(syscall.Errno); ok {
				return errNo == ERROR_PRIVILEGE_NOT_HELD
			}
		}
	}

	return false
}
