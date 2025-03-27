// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/progressbar"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/pkg/progress"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

type PushOptions struct {
	// Refspece: eg:
	//
	//  zeta push dev      // update branch or tag
	//  zeta push :dev     // delete branch or tag
	//  zeta push rev:dev  // update reference to rev
	//  zeta push          // update current branch
	Refspec     string
	PushObjects []string
	Tag         bool
	Force       bool
}

func (o *PushOptions) Target(name string) plumbing.ReferenceName {
	if strings.HasPrefix(name, plumbing.ReferencePrefix) {
		return plumbing.ReferenceName(name)
	}
	if o.Tag {
		return plumbing.NewTagReferenceName(name)
	}
	return plumbing.NewBranchReferenceName(name)
}

var (
	ErrPushRejected = errors.New("push rejected")
)

const (
	rejectFormat = "To %s\n" +
		"\x1b[31m! [rejected]\x1b[0m          %s -> %s (non-fast-forward)\n" +
		"\x1b[31merror: failed to push some ref to '%s'\x1b[0m\n" +
		"\x1b[33mhint: Updates were rejected because the tip of your current ref is behind\n" +
		"hint: its remote counterpart. Integrate the remote changes (e.g.\n" +
		"hint: 'zeta pull ...') before pushing again.\n" +
		"hint: See the 'Note about fast-forwards' in 'zeta push --help' for details.\x1b[0m\n"
)

func (r *Repository) putObject(ctx context.Context, t transport.Transport, refname plumbing.ReferenceName, oid plumbing.Hash, title string) error {
	sr, err := r.odb.SizeReader(oid, false)
	if err != nil {
		return err
	}
	defer sr.Close() // nolint
	var reader io.Reader = sr
	var b *progressbar.ProgressBar
	if !r.quiet {
		b = progressbar.NewOptions64(
			sr.Size(),
			progressbar.OptionShowBytes(true),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionUseANSICodes(true),
			progressbar.OptionSetDescription(title),
			progressbar.OptionFullWidth(),
			progressbar.OptionSetTheme(progress.MakeTheme()))
		reader = io.TeeReader(sr, b)
		defer b.Close() // nolint
	}
	if err = t.PutObject(ctx, refname, oid, reader, sr.Size()); err != nil {
		return err
	}
	return nil
}

func (r *Repository) putObjects(ctx context.Context, t transport.Transport, refname plumbing.ReferenceName, haveObjects []*transport.HaveObject) error {
	objects, err := t.BatchCheck(ctx, refname, haveObjects)
	if err != nil {
		return err
	}
	sendObjects := make([]*transport.HaveObject, 0, len(objects))
	for _, o := range objects {
		if o != nil && o.Action == transport.UPLOAD {
			sendObjects = append(sendObjects, &transport.HaveObject{OID: o.OID, CompressedSize: o.CompressedSize, Action: transport.UPLOAD})
		}
	}
	for i, o := range sendObjects {
		oid := plumbing.NewHash(o.OID)
		desc := fmt.Sprintf("%s \x1b[38;2;72;198;239m[%d/%d: %s]\x1b[0m", W("Upload Large files"), i+1, len(sendObjects), shortHash(oid))
		if err := r.putObject(ctx, t, refname, oid, desc); err != nil {
			return err
		}
	}
	return nil
}

func shortReferenceName(name plumbing.ReferenceName) string {
	if name.IsBranch() {
		return name.BranchName()
	}
	return string(name)
}

func (r *Repository) doPushRemove(ctx context.Context, target plumbing.ReferenceName, o *PushOptions) error {
	t, err := r.newTransport(ctx, transport.UPLOAD)
	if err != nil {
		return err
	}
	ref, err := t.FetchReference(ctx, target)
	cleanedRemote := r.cleanedRemote()
	if err == transport.ErrReferenceNotExist {
		die_error("unable to delete '%s': remote ref does not exist", shortReferenceName(target))
		die_error("failed to push some refs to '%s'", cleanedRemote)
		return err
	}
	if err != nil {
		die("ls-remote failed: %s", err)
		error_red("failed to push some refs to '%s'", cleanedRemote)
		return err
	}
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		defer pipeWriter.Close() // nolint
		if err := r.odb.PushTo(ctx, pipeWriter, &odb.PushObjects{
			Metadata:     make([]plumbing.Hash, 0),
			Objects:      make([]plumbing.Hash, 0),
			LargeObjects: make([]*odb.HaveObject, 0),
		}, r.quiet); err != nil {
			die("Push objects: %v", err)
			return
		}
	}()
	cmd := &transport.Command{
		Refname:     target,
		OldRev:      ref.Hash,
		NewRev:      plumbing.ZeroHash.String(),
		Metadata:    0,
		Objects:     0,
		PushOptions: o.PushObjects,
	}
	rc, err := t.Push(ctx, pipeReader, cmd)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		die_error("Push failed: %v", err)
		return err
	}
	result, err := r.odb.OnReport(ctx, target, rc)
	if err != nil {
		_ = rc.Close()
		if lastErr := rc.LastError(); lastErr != nil {
			die_error("Push failed: %v", lastErr)
			return lastErr
		}
		die_error("parse report error: %v", err)
		return err
	}
	_ = rc.Close()
	if result.Rejected {
		sv := strengthen.StrSplitSkipEmpty(result.Reason, 2, '\n')
		for _, s := range sv {
			_, _ = term.Fprintf(os.Stderr, "remote: %s\n", s)
		}
		_, _ = term.Fprintf(os.Stderr, "To: %s\n \x1b[31m! [remote rejected]\x1b[0m %s (delete)\n", cleanedRemote, target.Short())
		error_red("failed to push some refs to '%s'", cleanedRemote)
		return errors.New(result.Reason)
	}
	_, _ = fmt.Fprintf(os.Stderr, "To: %s\n - [deleted] '%s'\n", cleanedRemote, target.Short())
	return nil
}

func (r *Repository) doPush(ctx context.Context, ourName plumbing.ReferenceName, newRev plumbing.Hash, target plumbing.ReferenceName, o *PushOptions) error {
	t, err := r.newTransport(ctx, transport.UPLOAD)
	if err != nil {
		return err
	}
	var shallow plumbing.Hash
	var ignoreParents []plumbing.Hash
	if shallow, err = r.odb.DeepenFrom(); err != nil && !os.IsNotExist(err) {
		die("cat shallow error: %v", err)
		return err
	}
	if !shallow.IsZero() {
		shallowCommit, err := r.odb.Commit(ctx, shallow)
		if err != nil {
			die("read shallow commit %s error: %s", shallow, err)
			return err
		}
		ignoreParents = append(ignoreParents, shallowCommit.Parents...)
	}
	var fastForward, isNewPush bool
	var theirs, oldRev plumbing.Hash
	ref, err := t.FetchReference(ctx, target)
	switch {
	case err == transport.ErrReferenceNotExist:
		isNewPush = true
		if current, err := t.FetchReference(ctx, plumbing.HEAD); err == nil {
			theirs = plumbing.NewHash(current.Hash)
		}
	case err != nil:
		die("ls-remote '%s' error: %v", target, err)
		return err
	default:
		oldRev = plumbing.NewHash(ref.Hash)
		if newRev == oldRev {
			fmt.Fprintf(os.Stderr, "Everything up-to-date\n")
			return nil
		}
		// When updating a remote tag reference, if the remote reference is a tag object, you need to use --force to allow a push.
		if fastForward, err = r.isFastForward(ctx, oldRev, newRev, ignoreParents); err != nil {
			die("check is fast-forward %s error: %s", shallow, err)
			return err
		}
		cleanedRemote := r.cleanedRemote()
		if !fastForward && !o.Force {
			_, _ = term.Fprintf(os.Stderr, rejectFormat, cleanedRemote, ourName.Short(), ref.Name.Short(), cleanedRemote)
			return ErrPushRejected
		}
		theirs = ref.Target()
	}

	po, err := r.odb.Delta(ctx, newRev, shallow, theirs)
	if err != nil {
		die("get objects error: %v", err)
		return err
	}
	if len(po.LargeObjects) != 0 {
		haveObjects := make([]*transport.HaveObject, 0, len(po.LargeObjects))
		for _, o := range po.LargeObjects {
			haveObjects = append(haveObjects, &transport.HaveObject{OID: o.Hash.String(), CompressedSize: o.Size})
		}
		if err := r.putObjects(ctx, t, target, haveObjects); err != nil {
			die_error("upload large objects error: %v", err)
			return err
		}
	}
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		defer pipeWriter.Close() // nolint
		if err := r.odb.PushTo(ctx, pipeWriter, po, r.quiet); err != nil {
			return
		}
	}()
	cmd := &transport.Command{
		Refname:     target,
		OldRev:      plumbing.ZeroHash.String(),
		NewRev:      newRev.String(),
		Metadata:    len(po.Metadata),
		Objects:     len(po.Objects),
		PushOptions: o.PushObjects,
	}
	if ref != nil {
		cmd.OldRev = ref.Hash
	}
	rc, err := t.Push(ctx, pipeReader, cmd)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		die_error("Push failed: %v", err)
		return err
	}
	result, err := r.odb.OnReport(ctx, target, rc)
	if err != nil {
		_ = rc.Close()
		if lastErr := rc.LastError(); lastErr != nil {
			die_error("Push failed: %v", lastErr)
			return lastErr
		}
		die_error("parse report error: %v", err)
		return err
	}
	_ = rc.Close()
	cleanedRemote := r.cleanedRemote()
	if result.Rejected {
		sv := strengthen.StrSplitSkipEmpty(result.Reason, 2, '\n')
		for _, s := range sv {
			fmt.Fprintf(os.Stderr, "remote: %s\n", s)
		}
		_, _ = term.Fprintf(os.Stderr, "To: %s\n \x1b[31m! [remote rejected]\x1b[0m %s\n", cleanedRemote, target.Short())
		error_red("failed to push some refs to '%s'", cleanedRemote)
		return errors.New(result.Reason)
	}
	fmt.Fprintf(os.Stderr, "To: %s\n", cleanedRemote)
	if isNewPush {
		if target.IsBranch() {
			fmt.Fprintf(os.Stderr, " * [new branch] %s -> %s\n", ourName.Short(), target.BranchName())
			return nil
		}
		if target.IsTag() {
			fmt.Fprintf(os.Stderr, " * [new tag] %s -> %s\n", ourName.Short(), target.TagName())
			return nil
		}
		// not branch or tag skip
		return nil
	}
	if !fastForward {
		fmt.Fprintf(os.Stderr, " + %s...%s %s -> %s (forced update)\n", shortHash(oldRev), shortHash(newRev), ourName.Short(), shortReferenceName(target))
		return nil
	}
	fmt.Fprintf(os.Stderr, " + %s...%s %s -> %s\n", shortHash(oldRev), shortHash(newRev), ourName.Short(), shortReferenceName(target))
	return nil
}

func (r *Repository) Push(ctx context.Context, o *PushOptions) error {
	if len(o.Refspec) == 0 || o.Refspec == "HEAD" {
		current, err := r.Current()
		if err != nil {
			die("resolve HEAD error: %v", err)
			return err
		}
		return r.doPush(ctx, current.Name(), current.Hash(), current.Name(), o)
	}
	if ours, theirs, ok := strings.Cut(o.Refspec, ":"); ok {
		if len(ours) == 0 {
			// :target remove branch or tag
			return r.doPushRemove(ctx, o.Target(theirs), o)
		}
		newRev, err := r.Revision(ctx, ours)
		if err != nil {
			die("resolve %s error: %v", ours, err)
			return err
		}
		return r.doPush(ctx, plumbing.ReferenceName(ours), newRev, o.Target(theirs), o)
	}
	if strings.HasPrefix(o.Refspec, plumbing.ReferencePrefix) {
		refname := plumbing.ReferenceName(o.Refspec)
		ref, err := r.Reference(refname)
		if err != nil {
			die("resolve %s error: %v", o.Refspec, err)
			return err
		}
		return r.doPush(ctx, refname, ref.Hash(), ref.Name(), o)
	}
	ref, err := r.Reference(plumbing.NewBranchReferenceName(o.Refspec))
	if err == nil {
		return r.doPush(ctx, ref.Name(), ref.Hash(), ref.Name(), o)
	}
	if err != plumbing.ErrReferenceNotFound {
		die_error("resolve %s error: %v", o.Refspec, err)
		return err
	}
	if ref, err = r.Reference(plumbing.NewTagReferenceName(o.Refspec)); err != nil {
		die_error("unable resolve %s error: %v", o.Refspec, err)
		return err
	}
	return r.doPush(ctx, ref.Name(), ref.Hash(), ref.Name(), o)
}
