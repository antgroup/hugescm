// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/modules/zeta/refs"
)

// https://git-scm.com/docs/git-show-ref/zh_HANS-CN
// https://git-scm.com/docs/git-for-each-ref/zh_HANS-CN

type Reference struct {
	Name      plumbing.ReferenceName `json:"refname"`
	ShortName string                 `json:"refname_short"`
	Hash      plumbing.Hash          `json:"objectname"` // object name : commit/tag object ...
	Size      int                    `json:"objectsize"`
	Type      string                 `json:"objecttype"`
	Target    *object.Commit         `json:"commit,omitempty"`
	Tag       *object.Tag            `json:"tag,omitempty"`
	IsCurrent bool                   `json:"head,omitempty"`
}

func (r *Reference) commitdateLess(o *Reference) bool {
	return r.Target.Committer.When.Before(o.Target.Committer.When)
}

func (r *Reference) authordateLess(o *Reference) bool {
	return r.Target.Author.When.Before(o.Target.Author.When)
}

func (r *Reference) taggerdataLess(o *Reference) bool {
	if r.Tag == nil || o.Tag == nil {
		return r.Name < o.Name
	}
	return r.Tag.Tagger.When.Before(o.Tag.Tagger.When)
}

type References []*Reference

// objectsize authordate committerdate creatordate taggerdate
func ReferencesSort(rs References, order string) error {
	var reserve bool
	if strings.HasPrefix(order, "-") {
		reserve = true
		order = order[1:]
	}
	switch order {
	case "refname", "": // default refname
		if reserve {
			sort.Slice(rs, func(i, j int) bool {
				return rs[i].Name > rs[j].Name
			})
			return nil
		}
		sort.Slice(rs, func(i, j int) bool {
			return rs[i].Name < rs[j].Name
		})
		return nil
	case "authordate":
		if reserve {
			sort.Slice(rs, func(i, j int) bool {
				return !rs[i].authordateLess(rs[j])
			})
			return nil
		}
		sort.Slice(rs, func(i, j int) bool {
			return rs[i].authordateLess(rs[j])
		})
	case "committerdate":
		if reserve {
			sort.Slice(rs, func(i, j int) bool {
				return !rs[i].commitdateLess(rs[j])
			})
			return nil
		}
		sort.Slice(rs, func(i, j int) bool {
			return rs[i].commitdateLess(rs[j])
		})
		return nil
	case "taggerdate":
		if reserve {
			sort.Slice(rs, func(i, j int) bool {
				return !rs[i].taggerdataLess(rs[j])
			})
			return nil
		}
		sort.Slice(rs, func(i, j int) bool {
			return rs[i].taggerdataLess(rs[j])
		})
		return nil
	default:
	}
	return fmt.Errorf("unsupported references sort '%s'", order)
}

func (r *Repository) ReferenceExists(ctx context.Context, refname string) error {
	_, err := r.Reference(plumbing.ReferenceName(refname))
	if err == plumbing.ErrReferenceNotFound {
		die_error("reference does not exist")
		return err
	}
	if err != nil {
		die_error("open '%s': %v", refname, err)
		return err
	}
	return nil
}

type ForEachReferenceOptions struct {
	FormatJSON bool
	Order      string
	Pattern    []string
}

func (r *Repository) resolveReference(ctx context.Context, db *refs.DB, ref *plumbing.Reference) (*Reference, error) {
	o, err := r.odb.Object(ctx, ref.Hash())
	if err != nil {
		return nil, err
	}
	rr := &Reference{
		Name:      ref.Name(),
		Hash:      ref.Hash(),
		ShortName: db.ShortName(ref.Name(), true),
		IsCurrent: db.IsCurrent(ref.Name()),
	}
	switch a := o.(type) {
	case *object.Commit:
		rr.Target = a
		rr.Type = "commit"
		rr.Size = objectSize(a)
	case *object.Tag:
		rr.Tag = a
		rr.Type = "tag"
		rr.Size = objectSize(a)
		cc, err := r.odb.ParseRevExhaustive(ctx, a.Hash)
		if err != nil {
			return nil, err
		}
		rr.Target = cc
	default:
		return nil, fmt.Errorf("bad reference %s", ref.Name())
	}
	return rr, nil
}

func (r *Repository) ForEachReference(ctx context.Context, opts *ForEachReferenceOptions) error {
	m := NewMatcher(opts.Pattern)
	rdb, err := r.References()
	if err != nil {
		die_error("for-each-ref %v", err)
		return err
	}
	rs := make([]*Reference, 0, len(rdb.References()))
	for _, ref := range rdb.References() {
		if !m.Match(string(ref.Name())) {
			continue
		}
		rr, err := r.resolveReference(ctx, rdb, ref)
		if err != nil {
			return err
		}
		rs = append(rs, rr)
	}
	if err := ReferencesSort(rs, opts.Order); err != nil {
		return err
	}
	if opts.FormatJSON {
		return json.NewEncoder(os.Stdout).Encode(rs)
	}
	for _, ref := range rs {
		fmt.Fprintf(os.Stdout, "%s %s%s %s\n", ref.Hash, ref.Type, strings.Repeat(" ", max(0, len("commit")-len(ref.Type))), ref.Name)
	}
	return nil
}
