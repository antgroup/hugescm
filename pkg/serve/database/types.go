// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"regexp"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

const (
	DefaultBranch          = "mainline"
	DefaultCompressionALGO = "zstd"
	DefaultHashALGO        = "BLAKE3"
	DeletedSuffix          = ".deleted"
	Dot                    = "."
	DotDot                 = ".."
	DotZeta                = ".zeta"
)

// UserType defines the user type
type UserType int //revive:disable-line:exported

const (
	// UserTypeIndividual defines an individual user
	UserTypeIndividual UserType = iota

	// UserTypeBot defines a bot user
	UserTypeBot

	// UserTypeRemoteUser defines a remote user for federated users
	UserTypeRemoteUser
)

type User struct {
	ID             int64     `json:"id"`
	UserName       string    `json:"username"`
	Name           string    `json:"name"`
	Administrator  bool      `json:"administrator"`
	Email          string    `json:"email"`
	Type           UserType  `json:"type"`
	LockedAt       time.Time `json:"locked_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Password       string    `json:"-"`
	SignatureToken string    `json:"-"`
}

func (u *User) Guard() {
	u.Password = ""
}

type Branch struct {
	Name            string    `json:"name"`
	ID              int64     `json:"id"`
	RID             int64     `json:"rid"`
	Hash            string    `json:"hash"`
	ProtectionLevel int       `json:"protection_level"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Tag struct {
	Name        string    `json:"name"`
	RID         int64     `json:"rid"`
	UID         int64     `json:"uid"`
	Hash        string    `json:"hash"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Command struct {
	ReferenceName plumbing.ReferenceName `json:"reference_name"`
	OldRev        string                 `json:"old_rev"`
	NewRev        string                 `json:"new_rev"`
	Subject       string                 `json:"subject"`
	Description   string                 `json:"description"`
	RID           int64                  `json:"rid"`
	UID           int64                  `json:"uid"`
}

type Reference struct {
	ID              int64                  `json:"id"`
	Name            plumbing.ReferenceName `json:"name"`
	RID             int64                  `json:"rid"`
	Hash            string                 `json:"hash"`
	ProtectionLevel int                    `json:"protection_level"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// ^[a-zA-Z][a-zA-Z-_.]*((?<!.zeta)(?<!.deleted))$ start alpha
// ^(?!^[0-9]+$)(?!.*(?:.zeta|.deleted)$)[a-zA-Z0-9-_.]+$ no number
// ^(?!^[0-9]+$)(?!.*.deleted$)[a-zA-Z0-9-_.]+$

var (
	// GOLANG not support PCRE
	pathRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_\.]*$`)
)

func validatePath(p string) bool {
	return p != Dot && p != DotDot && !strings.HasSuffix(p, DeletedSuffix) && pathRegex.MatchString(p)
}

const (
	PrivateNamespace = 1
	GroupNamespace   = 2
)

type Namespace struct {
	ID          int64
	Path        string
	Name        string
	Owner       int64
	Type        int // 1-personal , 2-normal
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const (
	PrivateRepository   = 0
	InternalRepository  = 10
	PublicRepository    = 20
	AnonymousRepository = 30
)

type Repository struct {
	ID              int64     `json:"id"`
	NamespaceID     int64     `json:"namespace_id"`
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	Description     string    `json:"description"`
	VisibleLevel    int       `json:"visible_level"` //	0-private, 20-public, 30-anonymous
	DefaultBranch   string    `json:"default_branch"`
	HashAlgo        string    `json:"hash_algo"`
	CompressionAlgo string    `json:"compression_algo"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (r *Repository) IsPublic() bool {
	return r.VisibleLevel == PublicRepository
}

func (r *Repository) IsInternal() bool {
	return r.VisibleLevel == InternalRepository
}

func (r *Repository) Validate() error {
	if !validatePath(r.Path) {
		return &ErrNamingRule{name: r.Path}
	}
	if len(r.Name) == 0 {
		r.Name = r.Path
	}
	if len(r.DefaultBranch) == 0 {
		r.DefaultBranch = DefaultBranch
	}
	if len(r.CompressionAlgo) == 0 {
		r.CompressionAlgo = DefaultCompressionALGO
	}
	if len(r.HashAlgo) == 0 {
		r.HashAlgo = DefaultHashALGO
	}
	return nil
}

type KeyType int

func (t KeyType) String() string {
	switch t {
	case BasicKey:
		return "BasicKey"
	case DeployKey:
		return "DeployKey"
	}
	return "UnknownKey"
}

const (
	BasicKey KeyType = iota
	DeployKey
)

type Key struct {
	ID          int64     `json:"id"`
	UID         int64     `json:"uid"`
	Content     string    `json:"content"`
	Title       string    `json:"title"`
	Type        KeyType   `json:"type"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MemberType int

const (
	ProjectMember MemberType = 2
	GroupMember   MemberType = 3
)

type Member struct {
	ID          int64       `json:"id"`
	UID         int64       `json:"uid"`
	AccessLevel AccessLevel `json:"access_level"`
	SourceID    int64       `json:"source_id"`
	SourceType  MemberType  `json:"source_type"`
	ExpiresAt   time.Time   `json:"expires_at"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
