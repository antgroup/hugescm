// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package refs

import (
	"sort"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// DB: References DB
type DB struct {
	references []*plumbing.Reference
	cache      map[plumbing.ReferenceName]*plumbing.Reference
	head       *plumbing.Reference
}

func (d *DB) References() []*plumbing.Reference {
	return d.references
}

func (d *DB) Sort() {
	sort.Sort(plumbing.ReferenceSlice(d.references))
}

func (d *DB) HEAD() *plumbing.Reference {
	return d.head
}

func (d *DB) Lookup(name string) *plumbing.Reference {
	for _, r := range refRevParseRules {
		if r, ok := d.cache[r.ReferenceName(name)]; ok {
			return r
		}
	}
	return nil
}

func (d *DB) Resolve(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	for range MaxResolveRecursion {
		r := d.Lookup(string(name))
		if r == nil {
			return nil, plumbing.ErrReferenceNotFound
		}
		if r.Type() == plumbing.HashReference {
			return r, nil
		}
		if r.Type() != plumbing.SymbolicReference {
			return nil, plumbing.ErrReferenceNotFound
		}
	}
	return nil, plumbing.ErrReferenceNotFound
}

// Return shorten unambiguous refname
func (d *DB) ShortName(refname plumbing.ReferenceName, strict bool) string {
	for i := len(refRevParseRules) - 1; i > 0; i-- {
		var j int
		rulesToFail := 1
		shortName := refRevParseRules[i].ShortName(string(refname))
		if len(shortName) == 0 {
			continue
		}
		/*
		 * in strict mode, all (except the matched one) rules
		 * must fail to resolve to a valid non-ambiguous ref
		 */
		if strict {
			rulesToFail = len(refRevParseRules)
		}
		/*
		 * check if the short name resolves to a valid ref,
		 * but use only rules prior to the matched one
		 */
		for j = range rulesToFail {
			/* skip matched rule */
			if i == j {
				continue
			}
			/*
			 * the short name is ambiguous, if it resolves
			 * (with this previous rule) to a valid ref
			 * read_ref() returns 0 on success
			 */
			if d.Exists(refRevParseRules[j].ReferenceName(shortName)) {
				break
			}
		}
		/*
		 * short name is non-ambiguous if all previous rules
		 * haven't resolved to a valid ref
		 */
		if j == rulesToFail {
			return shortName
		}
	}
	return string(refname)
}

func (d *DB) Exists(refname plumbing.ReferenceName) bool {
	_, ok := d.cache[refname]
	return ok
}

func (d *DB) IsCurrent(refname plumbing.ReferenceName) bool {
	return d.head != nil && d.head.Name() == refname
}
