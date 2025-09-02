// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package replay

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/pkg/bar"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/trace"
)

// refUpdater is a type responsible for moving references from one point in the
// Git object graph to another.
type refUpdater struct {
	// CacheFn is a function that returns the SHA1 transformation from an
	// original hash to a new one. It specifies a "bool" return value
	// signaling whether or not that given "old" SHA1 was migrated.
	CacheFn func(old []byte) ([]byte, bool)
	// References is a set of *git.Ref's to migrate.
	References []*git.Reference
	// RepoPath is the given directory on disk in which the repository is
	// located.
	RepoPath string

	odb *git.ODB
}

// UpdateRefs performs the reference update(s) from existing locations (see:
// Refs) to their respective new locations in the graph (see CacheFn).
//
// It creates reflog entries as well as stderr log entries as it progresses
// through the reference updates.
//
// It returns any error encountered, or nil if the reference update(s) was/were
// successful.
func (r *refUpdater) UpdateRefs(ctx context.Context, b *bar.ProgressBar) error {

	var maxNameLen int
	for _, ref := range r.References {
		maxNameLen = max(maxNameLen, len(ref.Name))
	}

	seen := make(map[string]struct{})
	for _, ref := range r.References {
		if err := r.updateOneRef(ctx, maxNameLen, seen, ref); err != nil {
			return err
		}
		b.Add(1)
	}

	return nil
}

func (r *refUpdater) updateOneTag(tag *gitobj.Tag, toObj []byte) ([]byte, error) {
	newTag, err := r.odb.WriteTag(&gitobj.Tag{
		Object:     toObj,
		ObjectType: tag.ObjectType,
		Name:       tag.Name,
		Tagger:     tag.Tagger,

		Message: tag.Message,
	})

	if err != nil {
		return nil, fmt.Errorf("could not rewrite tag: %s", tag.Name)
	}
	return newTag, nil
}

func (r *refUpdater) rewriteTag(oid []byte) ([]byte, error) {
	tag, err := r.odb.Tag(oid)
	if err != nil {
		return nil, err
	}
	if tag.ObjectType == gitobj.TagObjectType {
		newTag, err := r.rewriteTag(tag.Object)
		if err != nil {
			return nil, err
		}
		return r.updateOneTag(tag, newTag)

	}
	if tag.ObjectType == gitobj.CommitObjectType {
		if to, ok := r.CacheFn(tag.Object); ok {
			return r.updateOneTag(tag, to)
		}
	}
	return oid, nil
}

func (r *refUpdater) updateOneRef(ctx context.Context, maxNameLen int, seen map[string]struct{}, ref *git.Reference) error {
	sha, err := hex.DecodeString(ref.Hash)
	if err != nil {
		return fmt.Errorf("could not decode: %q", ref.Hash)
	}
	if _, ok := seen[ref.Name]; ok {
		return nil
	}
	seen[ref.Name] = struct{}{}

	to, ok := r.CacheFn(sha)

	if ref.ObjectType == git.TagObject {
		newTag, err := r.rewriteTag(sha)
		if err != nil {
			return err
		}
		ok = !bytes.Equal(newTag, sha)
		to = newTag
	}

	if !ok {
		return nil
	}
	if err := git.ReferenceUpdate(ctx, r.RepoPath, ref.Name, "", hex.EncodeToString(to), true); err != nil {
		return err
	}

	namePadding := max(maxNameLen-len(ref.Name), 0)
	trace.DbgPrint("  %s%s\t%s -> %x", ref.Name, strings.Repeat(" ", namePadding), ref.Hash, to)
	return nil
}
