// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

// parseReflogRev: stash{0}, master{0}
func parseReflogRev(rev string) (string, int, error) {
	pos := strings.IndexByte(rev, '@')
	if pos == -1 {
		return "", 0, fmt.Errorf("'%s' not a valid reflog revision", rev)
	}
	refname := rev[:pos]
	s := rev[pos+1:]
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return "", 0, fmt.Errorf("'%s' not a valid reflog revision", rev)
	}
	depth, err := strconv.Atoi(s[1 : len(s)-1])
	return refname, depth, err
}

func resolveAncestor(revision string) (string, int, error) {
	if pos := strings.IndexByte(revision, '~'); pos != -1 {
		ns := revision[pos+1:]
		if len(ns) == 0 {
			return revision[0:pos], 1, nil
		}
		num, err := strconv.Atoi(ns)
		if err != nil {
			return "", 0, fmt.Errorf("not a valid object name %s", revision)
		}
		return revision[0:pos], num, nil
	}
	if pos := strings.IndexByte(revision, '^'); pos != -1 {
		for _, c := range revision[pos:] {
			if c != '^' {
				return "", 0, fmt.Errorf("not a valid object name %s", revision)
			}
		}
		return revision[0:pos], len(revision) - pos, nil
	}
	return revision, 0, nil
}

func newOID(s string) plumbing.Hash {
	if plumbing.ValidateHashHex(s) {
		return plumbing.NewHash(s)
	}
	return plumbing.ZeroHash
}

func (r *Repository) PickAncestor(ctx context.Context, oid plumbing.Hash, n int) (plumbing.Hash, error) {
	cur := oid
	for i := 0; i < n; i++ {
		cc, err := r.odb.ParseRevExhaustive(ctx, cur)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		if len(cc.Parents) == 0 {
			return plumbing.ZeroHash, nil
		}
		cur = cc.Parents[0]
	}
	return cur, nil
}

type ErrUnknownRevision struct {
	revision string
}

func (e *ErrUnknownRevision) Error() string {
	return fmt.Sprintf(W("ambiguous argument '%s': unknown revision or path not in the working tree."), e.revision)
}

func IsErrUnknownRevision(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrUnknownRevision)
	return ok
}

func (r *Repository) resolveRevision(ctx context.Context, revision string) (plumbing.Hash, error) {
	select {
	case <-ctx.Done():
		return plumbing.ZeroHash, ctx.Err()
	default:
	}
	if revision == string(plumbing.HEAD) {
		current, err := r.Current()
		if err != nil {
			return plumbing.ZeroHash, err
		}
		return current.Hash(), nil
	}
	if oid := newOID(revision); !oid.IsZero() {
		return oid, nil
	}
	if strings.HasPrefix(revision, plumbing.ReferencePrefix) {
		if ref, err := r.Reference(plumbing.ReferenceName(revision)); err == nil {
			return ref.Hash(), nil
		}
	}
	branch, err := r.Reference(plumbing.NewBranchReferenceName(revision))
	if err == nil {
		return branch.Hash(), nil
	}
	tag, err := r.Reference(plumbing.NewTagReferenceName(revision))
	if err == nil {
		return tag.Hash(), nil
	}
	if branchRemote, ok := strings.CutPrefix(revision, plumbing.Origin); ok {
		ref, err := r.Reference(plumbing.NewRemoteReferenceName(plumbing.Origin, branchRemote))
		if err == nil {
			return ref.Hash(), nil
		}
	}

	if len(revision) < 6 {
		return plumbing.ZeroHash, &ErrUnknownRevision{revision: revision}
	}
	rev, err := r.odb.Search(revision)
	if plumbing.IsNoSuchObject(err) {
		return plumbing.ZeroHash, &ErrUnknownRevision{revision: revision}
	}
	return rev, err
}

// Revision resolve revision
//
//	https://git-scm.com/book/en/v2/Git-Tools-Revision-Selection
//	We are not strictly compatible with Git, do not support combination mode, and do not support finding the second parent
//
// eg: HEAD HEAD^^^^ HEAD~2 BRANCH or TAG Long-OID Short-OID
func (r *Repository) Revision(ctx context.Context, branchOrTag string) (plumbing.Hash, error) {
	revision, ancestor, err := resolveAncestor(branchOrTag)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	oid, err := r.resolveRevision(ctx, revision)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if ancestor == 0 {
		return oid, nil
	}
	return r.PickAncestor(ctx, oid, ancestor)
}

func (r *Repository) tagTargetTree(ctx context.Context, tag *object.Tag, p string) (*object.Tree, error) {
	var cc *object.Commit
	var err error
	switch tag.ObjectType {
	case object.TagObject:
		cc, err = r.odb.ParseRevExhaustive(ctx, tag.Object)
	case object.CommitObject:
		cc, err = r.odb.Commit(ctx, tag.Object)
	default:
		return nil, backend.NewErrMismatchedObjectType(tag.Object, "commit")
	}
	if err != nil {
		return nil, err
	}
	root, err := cc.Root(ctx)
	if err != nil {
		return nil, err
	}
	e, err := root.FindEntry(ctx, p)
	if err != nil {
		return nil, err
	}
	if e.Type() != object.TreeObject {
		return nil, ErrNotTree
	}
	return r.odb.Tree(ctx, e.Hash)
}

var (
	ErrNotTree = errors.New("not tree")
)

func (r *Repository) readTree(ctx context.Context, oid plumbing.Hash, p string) (*object.Tree, error) {
	var err error
	var o any
	if o, err = r.odb.Object(ctx, oid); err != nil {
		if plumbing.IsNoSuchObject(err) && r.odb.Exists(oid, false) {
			return nil, ErrNotTree
		}
		return nil, err
	}
	switch a := o.(type) {
	case *object.Tag:
		return r.tagTargetTree(ctx, a, p)
	case *object.Tree:
		if len(p) == 0 {
			return a, nil
		}
		e, err := a.FindEntry(ctx, p)
		if err != nil {
			return nil, err
		}
		if e.Type() != object.TreeObject {
			return nil, ErrNotTree
		}
		return r.odb.Tree(ctx, e.Hash)
	case *object.Commit:
		root, err := r.odb.Tree(ctx, a.Tree)
		if err != nil {
			return nil, err
		}
		e, err := root.FindEntry(ctx, p)
		if err != nil {
			return nil, err
		}
		if e.Type() != object.TreeObject {
			return nil, ErrNotTree
		}
		return r.odb.Tree(ctx, e.Hash)
	}
	return nil, ErrNotTree
}

func (r *Repository) parseTargetEntry(ctx context.Context, tag *object.Tag, p string) (*object.TreeEntry, error) {
	var cc *object.Commit
	var err error
	switch tag.ObjectType {
	case object.TagObject:
		cc, err = r.odb.ParseRevExhaustive(ctx, tag.Object)
	case object.CommitObject:
		cc, err = r.odb.Commit(ctx, tag.Object)
	default:
		return nil, backend.NewErrMismatchedObjectType(tag.Object, "commit")
	}
	if err != nil {
		return nil, err
	}
	root, err := cc.Root(ctx)
	if err != nil {
		return nil, err
	}
	if len(p) == 0 {
		return &object.TreeEntry{Hash: root.Hash, Mode: filemode.Dir}, nil
	}
	return root.FindEntry(ctx, p)
}

func (r *Repository) parseEntry(ctx context.Context, oid plumbing.Hash, p string) (*object.TreeEntry, error) {
	var err error
	var o any
	if o, err = r.odb.Object(ctx, oid); err != nil {
		if plumbing.IsNoSuchObject(err) && r.odb.Exists(oid, false) {
			return &object.TreeEntry{Hash: oid, Name: oid.String(), Mode: filemode.Regular}, nil
		}
		return nil, err
	}
	switch a := o.(type) {
	case *object.Tag:
		return r.parseTargetEntry(ctx, a, p)
	case *object.Tree:
		if len(p) == 0 {
			return &object.TreeEntry{Hash: a.Hash, Mode: filemode.Dir, Name: a.Hash.String()}, nil
		}
		return a.FindEntry(ctx, p)
	case *object.Commit:
		root, err := r.odb.Tree(ctx, a.Tree)
		if err != nil {
			return nil, err
		}
		if len(p) == 0 {
			return &object.TreeEntry{Hash: root.Hash, Mode: filemode.Dir, Name: root.Hash.String()}, nil
		}
		return root.FindEntry(ctx, p)
	case *object.Fragments:
		return &object.TreeEntry{Hash: oid, Name: p, Mode: filemode.Regular | filemode.Fragments}, nil
	}
	return nil, ErrNotTree
}

func (r *Repository) parseTreeEntryExhaustive(ctx context.Context, branchOrTag string) (*object.TreeEntry, string, error) {
	prefix, p, ok := strings.Cut(branchOrTag, ":")
	oid, err := r.Revision(ctx, prefix)
	if err != nil {
		return nil, "", err
	}
	e, err := r.parseEntry(ctx, oid, p)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		p = prefix
	}
	return e, p, nil
}

func (r *Repository) parseTreeExhaustive(ctx context.Context, branchOrTag string) (*object.Tree, error) {
	prefix, p, _ := strings.Cut(branchOrTag, ":")
	oid, err := r.Revision(ctx, prefix)
	if err != nil {
		return nil, err
	}
	return r.readTree(ctx, oid, p)
}

func (r *Repository) parseRevExhaustive(ctx context.Context, branchOrTag string) (*object.Commit, error) {
	oid, err := r.Revision(ctx, branchOrTag)
	if err != nil {
		return nil, err
	}
	return r.odb.ParseRevExhaustive(ctx, oid)
}

func (r *Repository) IsCurrent(refname plumbing.ReferenceName) bool {
	current, err := r.Current()
	if err != nil {
		return false
	}
	return current.Name() == refname
}

func (r *Repository) RevisionEx(ctx context.Context, revision string) (plumbing.Hash, plumbing.ReferenceName, error) {
	if revision == string(plumbing.HEAD) {
		current, err := r.Current()
		if err != nil {
			return plumbing.ZeroHash, "", err
		}
		return current.Hash(), current.Name(), nil
	}
	if oid := newOID(revision); !oid.IsZero() {
		return oid, "", nil
	}
	if strings.HasPrefix(revision, plumbing.ReferencePrefix) {
		if ref, err := r.Reference(plumbing.ReferenceName(revision)); err == nil {
			return ref.Hash(), ref.Name(), nil
		}
	}
	branch, err := r.Reference(plumbing.NewBranchReferenceName(revision))
	if err == nil {
		return branch.Hash(), branch.Name(), nil
	}
	tag, err := r.Reference(plumbing.NewTagReferenceName(revision))
	if err == nil {
		return tag.Hash(), tag.Name(), nil
	}

	if len(revision) < 6 {
		return plumbing.ZeroHash, "", &ErrUnknownRevision{revision: revision}
	}
	rev, err := r.odb.Search(revision)
	if err != nil && plumbing.IsNoSuchObject(err) {
		return plumbing.ZeroHash, "", &ErrUnknownRevision{revision: revision}
	}
	return rev, "", err
}

func (r *Repository) isFastForward(ctx context.Context, oldRev, newRev plumbing.Hash, ignore []plumbing.Hash) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	c, err := r.odb.ParseRevExhaustive(ctx, newRev)
	if err != nil {
		return false, err
	}
	found := false
	// stop iterating at the earlist shallow commit, ignoring its parents
	// note: when pull depth is smaller than the number of new changes on the remote, this fails due to missing parents.
	//       as far as i can tell, without the commits in-between the shallow pull and the earliest shallow, there's no
	//       real way of telling whether it will be a fast-forward merge.
	iter := object.NewCommitPreorderIter(c, nil, ignore)
	err = iter.ForEach(ctx, func(c *object.Commit) error {
		if c.Hash != oldRev {
			return nil
		}

		found = true
		return plumbing.ErrStop
	})
	return found, err
}

func (r *Repository) IsAncestor(ctx context.Context, a, b string) error {
	rev1, err := r.parseRevExhaustive(ctx, a)
	if err != nil {
		die_error("merge-base: parse %s: %v", a, err)
		return err
	}
	rev2, err := r.parseRevExhaustive(ctx, b)
	if err != nil {
		die_error("merge-base: parse %s: %v", b, err)
		return err
	}
	bases, err := rev1.MergeBase(ctx, rev2)
	if err != nil {
		die_error("merge-base error: %v", err)
		return err
	}
	if len(bases) == 0 {
		fmt.Fprintln(os.Stderr, "merge-base: unrelated histories")
		return ErrUnrelatedHistories
	}
	for _, b := range bases {
		if b.Hash == rev1.Hash {
			return nil
		}
	}
	return ErrNotAncestor
}

func (r *Repository) MergeBase(ctx context.Context, revisions []string, all bool) error {
	commits := make([]*object.Commit, 0, len(revisions))
	for _, a := range revisions {
		cc, err := r.parseRevExhaustive(ctx, a)
		if err != nil {
			die_error("merge-base: parse %s: %v", a, err)
			return err
		}
		commits = append(commits, cc)
	}
	if len(commits) < 2 {
		die_error("merge-base: bad arguments missing commits")
		return ErrAborting
	}
	c0 := commits[0]
	bases := make([]*object.Commit, 0, 2)
	var err error
	current := c0
	for i := 1; i < len(commits); i++ {
		rev := commits[i]
		if bases, err = rev.MergeBase(ctx, current); err != nil {
			die_error("merge-base: %v", err)
			return err
		}
		if len(bases) == 0 {
			fmt.Fprintln(os.Stderr, "merge-base: unrelated histories")
			return ErrUnrelatedHistories
		}
		current = bases[0]
	}
	if all {
		for _, b := range bases {
			fmt.Fprintln(os.Stdout, b.Hash)
		}
		return nil
	}
	fmt.Fprintln(os.Stdout, bases[0].Hash)
	return nil
}
