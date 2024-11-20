// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package refs

import (
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// ReferencePrefixMatch: follow git's priority for finding refs
//
// https://git-scm.com/docs/git-rev-parse#Documentation/git-rev-parse.txt-emltrefnamegtemegemmasterememheadsmasterememrefsheadsmasterem
//
// https://github.com/git/git/blob/master/Documentation/revisions.txt

type Rule struct {
	prefix string
	suffix string
}

func (r Rule) ReferenceName(name string) plumbing.ReferenceName {
	return plumbing.ReferenceName(r.prefix + name + r.suffix)
}

func (r Rule) ShortName(name string) string {
	if strings.HasPrefix(name, r.prefix) {
		return strings.TrimSuffix(name[len(r.prefix):], r.suffix)
	}
	return ""
}

var (
	refRevParseRules = []*Rule{
		{},
		{prefix: "refs/"},
		{prefix: "refs/tags/"},
		{prefix: "refs/heads/"},
		{prefix: "refs/remotes/"},
		{prefix: "refs/remotes/", suffix: "/HEAD"},
	}
)

// RefRevParseRules are a set of rules to parse references into short names.
// These are the same rules as used by git in shorten_unambiguous_ref.
// See: https://github.com/git/git/blob/9857273be005833c71e2d16ba48e193113e12276/refs.c#L610
func RefRevParseRules() []*Rule {
	return refRevParseRules
}
