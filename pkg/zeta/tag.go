// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/modules/zeta/refs"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/mattn/go-isatty"
)

func (r *Repository) RemoveTag(tags []string) error {
	for _, t := range tags {
		ref, err := r.Reference(plumbing.NewTagReferenceName(t))
		if err == plumbing.ErrReferenceNotFound {
			die_error("tag '%s' not found.", t)
			return err
		}
		if err != nil {
			die_error("find tag %s: %v", t, err)
			return err
		}
		if err := r.ReferenceRemove(ref); err != nil {
			die_error("remove tag: %v", err)
			return err
		}
		fmt.Fprintf(os.Stderr, "Deleted tag '%s' (was %s)\n", t, shortHash(ref.Hash()))
	}
	return nil
}

func (r *Repository) ListTag(ctx context.Context, pattern []string) error {
	db, err := refs.ReferencesDB(r.zetaDir)
	if err != nil {
		die_error("references db error: %v", err)
		return err
	}
	m := NewMatcher(pattern)
	w := NewPrinter(ctx)
	defer w.Close()
	for _, r := range db.References() {
		if !r.Name().IsTag() {
			continue
		}
		refname := r.Name().TagName()
		if !m.Match(refname) {
			continue
		}
		fmt.Fprintf(w, "  %s\n", refname)
	}
	return nil
}

type NewTagOptions struct {
	Name     string
	Target   string
	Message  []string
	File     string
	Annotate bool
	Force    bool
}

func (r *Repository) tagMessageFromPrompt(ctx context.Context, opts *NewTagOptions, oldRef *plumbing.Reference) (string, error) {
	if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) && !env.ZETA_TERMINAL_PROMPT.SimpleAtob(true) {
		return "", nil
	}
	p := filepath.Join(r.odb.Root(), TAG_EDITMSG)
	var b bytes.Buffer
	if oldRef != nil {
		if tag, err := r.odb.Tag(ctx, oldRef.Hash()); err == nil {
			for _, s := range strings.Split(tag.Content, "\n") {
				fmt.Fprintf(&b, "%s\n", s)
			}
		}
	} else {
		_ = b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "#\n# %s\n#   %s\n# %s\n", W("Write a message for tag:"), opts.Name, tr.Sprintf("Lines starting with '%c' will be ignored.", '#'))
	if err := os.WriteFile(p, b.Bytes(), 0644); err != nil {
		return "", err
	}
	if err := launchEditor(ctx, r.coreEditor(), p, nil); err != nil {
		return "", nil
	}
	return messageReadFromPath(p)
}

func (r *Repository) NewTag(ctx context.Context, opts *NewTagOptions) error {
	if !plumbing.ValidateTagName([]byte(opts.Name)) {
		die("'%s' is not a valid tag name.", opts.Name)
		return &plumbing.ErrBadReferenceName{Name: opts.Name}
	}
	tagName := plumbing.NewTagReferenceName(opts.Name)
	oldRef, err := r.ReferencePrefixMatch(tagName)
	switch {
	case err == nil:
		if oldRef.Name() != tagName {
			die("'%s' exists; cannot create '%s'", oldRef.Name(), tagName)
			return errors.New("tag exists")
		}
		if !opts.Force {
			die("tag '%s' already exists", opts.Name)
			return errors.New("tag exists")
		}
	case err == plumbing.ErrReferenceNotFound:
	default:
		die_error("find tag: %v", err)
	}
	var message string
	switch {
	case opts.File == "-":
		if message, err = messageReadFrom(os.Stdin); err != nil {
			die("read message from stdin: %v", err)
			return err
		}
	case len(opts.File) != 0:
		if message, err = messageReadFromPath(opts.File); err != nil {
			die("read message from %s: %v", opts.File, err)
			return err
		}
	case len(opts.Message) == 0 && opts.Annotate:
		if message, err = r.tagMessageFromPrompt(ctx, opts, oldRef); err != nil {
			die("read message from prompt: %v", err)
			return err
		}
	default:
		message = genMessage(opts.Message)
	}
	annotate := opts.Annotate
	if len(opts.Message) != 0 || len(opts.File) != 0 {
		annotate = true
	}
	if annotate && len(message) == 0 {
		die("no tag message?")
		return ErrNotAllowEmptyMessage
	}

	rev, err := r.parseRevExhaustive(ctx, opts.Target)
	if err != nil {
		die_error("resolve %s: %v", opts.Target, err)
		return err
	}
	newRev := rev.Hash
	if annotate {
		singature := r.NewCommitter()
		tag := &object.Tag{
			Object:     rev.Hash,
			ObjectType: object.CommitObject,
			Name:       opts.Name,
			Tagger:     *singature,
			Content:    message,
		}
		if newRev, err = r.odb.WriteEncoded(tag); err != nil {
			die_error("encode tag object: %v", err)
			return err
		}
	}
	newRef := plumbing.NewHashReference(tagName, newRev)
	if err := r.ReferenceUpdate(newRef, oldRef); err != nil {
		die_error("update-ref: %v", err)
		return err
	}
	return nil
}
