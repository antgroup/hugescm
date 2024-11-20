// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

// DoUpdate: update-ref
func (r *Repository) DoUpdate(ctx context.Context, refname plumbing.ReferenceName, oldRev, newRev plumbing.Hash, committer *object.Signature, message string) error {
	if newRev.IsZero() {
		if err := r.ReferenceRemove(plumbing.NewHashReference(refname, oldRev)); err != nil {
			return err
		}
		if err := r.rdb.Delete(refname); err != nil {
			r.DbgPrint("delete reflog: %v", err)
		}
		return nil
	}
	var old *plumbing.Reference
	if !oldRev.IsZero() {
		old = plumbing.NewHashReference(refname, oldRev)
	}
	if err := r.ReferenceUpdate(plumbing.NewHashReference(refname, newRev), old); err != nil {
		return err
	}
	if oldRev == newRev {
		return nil
	}
	ro, err := r.rdb.Read(refname)
	if err != nil {
		return nil
	}
	ro.Push(newRev, committer, message)
	if err = r.rdb.Write(ro); err != nil {
		r.DbgPrint("reflog: %v", err)
	}
	return nil
}

func (r *Repository) writeHEADReflog(newRev plumbing.Hash, committer *object.Signature, message string) error {
	ro, err := r.rdb.Read(plumbing.HEAD)
	if err != nil {
		return nil
	}
	ro.Push(newRev, committer, message)
	if err = r.rdb.Write(ro); err != nil {
		r.DbgPrint("reflog: %v", err)
	}
	return nil
}
