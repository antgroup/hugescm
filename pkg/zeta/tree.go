// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"context"
	"errors"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

type CommitTreeOptions struct {
	Tree plumbing.Hash
	// Author is the author's signature of the commit. If Author is empty the
	// Name and Email is read from the config, and time.Now it's used as When.
	Author object.Signature
	// Committer is the committer's signature of the commit. If Committer is
	// nil the Author signature is used.
	Committer object.Signature
	// Parents are the parents commits for the new commit, by default when
	// len(Parents) is zero, the hash of HEAD reference is used.
	Parents []plumbing.Hash
	// SignKey denotes a key to sign the commit with. A nil value here means the
	// commit will not be signed. The private key must be present and already
	// decrypted.
	SignKey *openpgp.Entity
	// Amend will create a new commit object and replace the commit that HEAD currently
	// points to. Cannot be used with All nor Parents.
	Message string
}

func (r *Repository) CommitTree(ctx context.Context, opts *CommitTreeOptions) (plumbing.Hash, error) {
	if !r.odb.Exists(opts.Tree, false) {
		return plumbing.ZeroHash, plumbing.NoSuchObject(opts.Tree)
	}
	for _, p := range opts.Parents {
		if p.IsZero() {
			return plumbing.ZeroHash, errors.New("bad object")
		}
	}
	return r.commitTree(ctx, opts)
}

func (r *Repository) commitTree(ctx context.Context, opts *CommitTreeOptions) (plumbing.Hash, error) {
	select {
	case <-ctx.Done():
		return plumbing.ZeroHash, ctx.Err()
	default:
	}
	commit := &object.Commit{
		Author:    opts.Author,
		Committer: opts.Committer,
		Message:   opts.Message,
		Tree:      opts.Tree,
		Parents:   opts.Parents,
	}

	if opts.SignKey != nil {
		sig, err := buildCommitSignature(commit, opts.SignKey)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		commit.ExtraHeaders = append(commit.ExtraHeaders, &object.ExtraHeader{
			K: "gpgsig",
			V: sig,
		})
	}
	if oid := object.Hash(commit); r.odb.Exists(oid, true) {
		return oid, nil
	}
	return r.odb.WriteEncoded(commit)
}

func buildCommitSignature(commit *object.Commit, signKey *openpgp.Entity) (string, error) {
	var encoded bytes.Buffer
	if err := commit.Encode(&encoded); err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&b, signKey, bytes.NewReader(encoded.Bytes()), nil); err != nil {
		return "", err
	}
	return b.String(), nil
}
