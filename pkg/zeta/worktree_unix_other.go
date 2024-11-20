// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

//go:build openbsd || dragonfly || solaris
// +build openbsd dragonfly solaris

package zeta

import (
	"syscall"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

const (
	escapeChars = "*?[]\\"
)

func init() {
	fillSystemInfo = func(e *index.Entry, sys any) {
		if os, ok := sys.(*syscall.Stat_t); ok {
			e.CreatedAt = time.Unix(os.Atim.Unix())
			e.Dev = uint32(os.Dev)
			e.Inode = uint32(os.Ino)
			e.GID = os.Gid
			e.UID = os.Uid
		}
	}
}

func isSymlinkWindowsNonAdmin(error) bool {
	return false
}
