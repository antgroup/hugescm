package plumbing

import (
	"errors"
	"fmt"
	"strings"
)

const (
	ReferencePrefix = "refs/"
	refHeadPrefix   = ReferencePrefix + "heads/"
	refTagPrefix    = ReferencePrefix + "tags/"
	refRemotePrefix = ReferencePrefix + "remotes/"
	symrefPrefix    = "ref: "
)

const (
	Origin = "origin"
)

const (
	RefRevParseRulesCount = 6
)

// RefRevParseRules are a set of rules to parse references into short names.
// These are the same rules as used by git in shorten_unambiguous_ref.
// See: https://github.com/git/git/blob/9857273be005833c71e2d16ba48e193113e12276/refs.c#L610
var RefRevParseRules = []string{
	"%s",
	"refs/%s",
	"refs/tags/%s",
	"refs/heads/%s",
	"refs/remotes/%s",
	"refs/remotes/%s/HEAD",
}

var (
	ErrReferenceNotFound = errors.New("reference does not exist")
)

// ReferenceType reference type's
type ReferenceType int8

const (
	InvalidReference  ReferenceType = 0
	HashReference     ReferenceType = 1
	SymbolicReference ReferenceType = 2
)

func (r ReferenceType) String() string {
	switch r {
	case InvalidReference:
		return "invalid-reference"
	case HashReference:
		return "hash-reference"
	case SymbolicReference:
		return "symbolic-reference"
	}

	return ""
}

// ReferenceName reference name's
type ReferenceName string

// NewBranchReferenceName returns a reference name describing a branch based on
// his short name.
func NewBranchReferenceName(name string) ReferenceName {
	return ReferenceName(refHeadPrefix + name)
}

// NewRemoteReferenceName returns a reference name describing a remote branch
// based on his short name and the remote name.
func NewRemoteReferenceName(remote, name string) ReferenceName {
	return ReferenceName(refRemotePrefix + fmt.Sprintf("%s/%s", remote, name))
}

// NewRemoteHEADReferenceName returns a reference name describing a the HEAD
// branch of a remote.
func NewRemoteHEADReferenceName(remote string) ReferenceName {
	return ReferenceName(refRemotePrefix + fmt.Sprintf("%s/%s", remote, HEAD))
}

// NewTagReferenceName returns a reference name describing a tag based on short
// his name.
func NewTagReferenceName(name string) ReferenceName {
	return ReferenceName(refTagPrefix + name)
}

// IsBranch check if a reference is a branch
func (r ReferenceName) IsBranch() bool {
	return strings.HasPrefix(string(r), refHeadPrefix)
}

func (r ReferenceName) BranchName() string {
	return strings.TrimPrefix(string(r), refHeadPrefix)
}

// IsRemote check if a reference is a remote
func (r ReferenceName) IsRemote() bool {
	return strings.HasPrefix(string(r), refRemotePrefix)
}

// IsTag check if a reference is a tag
func (r ReferenceName) IsTag() bool {
	return strings.HasPrefix(string(r), refTagPrefix)
}

func (r ReferenceName) TagName() string {
	return strings.TrimPrefix(string(r), refTagPrefix)
}

func (r ReferenceName) String() string {
	return string(r)
}

// Short returns the short name of a ReferenceName
//
//	un strict, does not check whether the name is ambiguous
func (r ReferenceName) Short() string {
	s := string(r)
	res := s
	// skip first
	for _, format := range RefRevParseRules[1:] {
		_, err := fmt.Sscanf(s, format, &res)
		if err == nil {
			continue
		}
	}

	return res
}

func (r ReferenceName) Prefix() string {
	if r.IsBranch() {
		return "refs/heads"
	}
	if r.IsTag() {
		return "refs/tags"
	}
	if r.IsRemote() {
		return "refs/remotes"
	}
	return string(r)
}

const (
	HEAD     ReferenceName = "HEAD"
	Mainline ReferenceName = "refs/heads/mainline"
)

// Reference is a representation of git reference
type Reference struct {
	t      ReferenceType
	n      ReferenceName
	h      Hash
	target ReferenceName
}

// NewReferenceFromStrings creates a reference from name and target as string,
// the resulting reference can be a SymbolicReference or a HashReference base
// on the target provided
func NewReferenceFromStrings(name, target string) *Reference {
	n := ReferenceName(name)

	if strings.HasPrefix(target, symrefPrefix) {
		target := ReferenceName(target[len(symrefPrefix):])
		return NewSymbolicReference(n, target)
	}

	return NewHashReference(n, NewHash(target))
}

// NewSymbolicReference creates a new SymbolicReference reference
func NewSymbolicReference(n, target ReferenceName) *Reference {
	return &Reference{
		t:      SymbolicReference,
		n:      n,
		target: target,
	}
}

// NewHashReference creates a new HashReference reference
func NewHashReference(n ReferenceName, h Hash) *Reference {
	return &Reference{
		t: HashReference,
		n: n,
		h: h,
	}
}

// Type returns the type of a reference
func (r *Reference) Type() ReferenceType {
	return r.t
}

// Name returns the name of a reference
func (r *Reference) Name() ReferenceName {
	return r.n
}

// Hash returns the hash of a hash reference
func (r *Reference) Hash() Hash {
	return r.h
}

// Target returns the target of a symbolic reference
func (r *Reference) Target() ReferenceName {
	return r.target
}

// Strings dump a reference as a [2]string
func (r *Reference) Strings() [2]string {
	var o [2]string
	o[0] = r.Name().String()

	switch r.Type() {
	case HashReference:
		o[1] = r.h.String()
	case SymbolicReference:
		o[1] = symrefPrefix + r.Target().String()
	}

	return o
}

func (r *Reference) String() string {
	ref := ""
	switch r.Type() {
	case HashReference:
		ref = r.h.String()
	case SymbolicReference:
		ref = symrefPrefix + r.Target().String()
	default:
		return ""
	}

	name := r.Name().String()
	var v strings.Builder
	v.Grow(len(ref) + len(name) + 1)
	v.WriteString(ref)
	v.WriteString(" ")
	v.WriteString(name)
	return v.String()
}

type ReferenceSlice []*Reference

func (p ReferenceSlice) Len() int           { return len(p) }
func (p ReferenceSlice) Less(i, j int) bool { return p[i].Name() < p[j].Name() }
func (p ReferenceSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
