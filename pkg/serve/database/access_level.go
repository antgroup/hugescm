// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

type AccessLevel int

const (
	NoneAccess     AccessLevel = 0
	ReporterAccess AccessLevel = 20
	DevAccess      AccessLevel = 30
	MasterAccess   AccessLevel = 40
	OwnerAccess    AccessLevel = 50
)

func (accessLevel AccessLevel) Writeable() bool {
	return accessLevel >= DevAccess
}

func (accessLevel AccessLevel) Readable() bool {
	return accessLevel >= ReporterAccess
}

func (accessLevel AccessLevel) Sudo() bool {
	return accessLevel >= MasterAccess
}
